package node

import (
	"encoding/gob"
	"fmt"
	"github.com/benbjohnson/clock"
	"github.com/lf-edge/ekuiper/internal/conf"
	"github.com/lf-edge/ekuiper/internal/xsql"
	"github.com/lf-edge/ekuiper/pkg/api"
	"github.com/lf-edge/ekuiper/pkg/ast"
	"github.com/lf-edge/ekuiper/pkg/cast"
	"math"
	"time"
)

type WindowConfig struct {
	Type     ast.WindowType
	Length   int
	Interval int //If interval is not set, it is equals to Length
}

type WindowOperator struct {
	*defaultSinkNode
	window             *WindowConfig
	interval           int
	isEventTime        bool
	watermarkGenerator *WatermarkGenerator //For event time only

	statManager StatManager
	ticker      *clock.Ticker //For processing time only
	// states
	triggerTime int64
	msgCount    int
}

const WINDOW_INPUTS_KEY = "$$windowInputs"
const TRIGGER_TIME_KEY = "$$triggerTime"
const MSG_COUNT_KEY = "$$msgCount"

func init() {
	gob.Register([]*xsql.Tuple{})
}

func NewWindowOp(name string, w WindowConfig, streams []string, options *api.RuleOption) (*WindowOperator, error) {
	o := new(WindowOperator)

	o.defaultSinkNode = &defaultSinkNode{
		input: make(chan interface{}, options.BufferLength),
		defaultNode: &defaultNode{
			outputs:   make(map[string]chan<- interface{}),
			name:      name,
			sendError: options.SendError,
		},
	}
	o.isEventTime = options.IsEventTime
	o.window = &w
	if o.window.Interval == 0 && o.window.Type == ast.COUNT_WINDOW {
		//if no interval value is set and it's count window, then set interval to length value.
		o.window.Interval = o.window.Length
	}
	if options.IsEventTime {
		//Create watermark generator
		if w, err := NewWatermarkGenerator(o.window, options.LateTol, streams, o.input); err != nil {
			return nil, err
		} else {
			o.watermarkGenerator = w
		}
	}
	return o, nil
}

// Exec is the entry point for the executor
// input: *xsql.Tuple from preprocessor
// output: xsql.WindowTuplesSet
func (o *WindowOperator) Exec(ctx api.StreamContext, errCh chan<- error) {
	o.ctx = ctx
	log := ctx.GetLogger()
	log.Debugf("Window operator %s is started", o.name)

	if len(o.outputs) <= 0 {
		go func() { errCh <- fmt.Errorf("no output channel found") }()
		return
	}
	stats, err := NewStatManager("op", ctx)
	if err != nil {
		go func() { errCh <- err }()
		return
	}
	o.statManager = stats
	var inputs []*xsql.Tuple
	if s, err := ctx.GetState(WINDOW_INPUTS_KEY); err == nil {
		switch st := s.(type) {
		case []*xsql.Tuple:
			inputs = st
			log.Infof("Restore window state %+v", inputs)
		case nil:
			log.Debugf("Restore window state, nothing")
		default:
			errCh <- fmt.Errorf("restore window state `inputs` %v error, invalid type", st)
		}
	} else {
		log.Warnf("Restore window state fails: %s", err)
	}
	o.triggerTime = conf.GetNowInMilli()
	if s, err := ctx.GetState(TRIGGER_TIME_KEY); err == nil && s != nil {
		if si, ok := s.(int64); ok {
			o.triggerTime = si
		} else {
			errCh <- fmt.Errorf("restore window state `triggerTime` %v error, invalid type", s)
		}
	}
	o.msgCount = 0
	if s, err := ctx.GetState(MSG_COUNT_KEY); err == nil && s != nil {
		if si, ok := s.(int); ok {
			o.msgCount = si
		} else {
			errCh <- fmt.Errorf("restore window state `msgCount` %v error, invalid type", s)
		}
	}
	log.Infof("Start with window state triggerTime: %d, msgCount: %d", o.triggerTime, o.msgCount)
	if o.isEventTime {
		go o.execEventWindow(ctx, inputs, errCh)
	} else {
		go o.execProcessingWindow(ctx, inputs, errCh)
	}
}

func (o *WindowOperator) execProcessingWindow(ctx api.StreamContext, inputs []*xsql.Tuple, errCh chan<- error) {
	log := ctx.GetLogger()
	var (
		c             <-chan time.Time
		timeoutTicker *clock.Timer
		timeout       <-chan time.Time
	)
	switch o.window.Type {
	case ast.NOT_WINDOW:
	case ast.TUMBLING_WINDOW:
		o.ticker = conf.GetTicker(o.window.Length)
		o.interval = o.window.Length
	case ast.HOPPING_WINDOW:
		o.ticker = conf.GetTicker(o.window.Interval)
		o.interval = o.window.Interval
	case ast.SLIDING_WINDOW:
		o.interval = o.window.Length
	case ast.SESSION_WINDOW:
		o.ticker = conf.GetTicker(o.window.Length)
		o.interval = o.window.Interval
	case ast.COUNT_WINDOW:
		o.interval = o.window.Interval
	}

	if o.ticker != nil {
		c = o.ticker.C
		//resume previous window
		if len(inputs) > 0 && o.triggerTime > 0 {
			nextTick := conf.GetNowInMilli() + int64(o.interval)
			next := o.triggerTime
			switch o.window.Type {
			case ast.TUMBLING_WINDOW, ast.HOPPING_WINDOW:
				for {
					next = next + int64(o.interval)
					if next > nextTick {
						break
					}
					log.Debugf("triggered by restore inputs")
					inputs, _ = o.scan(inputs, next, ctx)
					ctx.PutState(WINDOW_INPUTS_KEY, inputs)
					ctx.PutState(TRIGGER_TIME_KEY, o.triggerTime)
				}
			case ast.SESSION_WINDOW:
				timeout, duration := int64(o.window.Interval), int64(o.window.Length)
				for {
					et := inputs[0].Timestamp
					tick := et + (duration - et%duration)
					if et%duration == 0 {
						tick = et
					}
					var p int64
					for _, tuple := range inputs {
						var r int64 = math.MaxInt64
						if p > 0 {
							if tuple.Timestamp-p > timeout {
								r = p + timeout
							}
						}
						if tuple.Timestamp > tick {
							if tick-duration > et && tick < r {
								r = tick
							}
							tick += duration
						}
						if r < math.MaxInt64 {
							next = r
							break
						}
						p = tuple.Timestamp
					}
					if next > nextTick {
						break
					}
					log.Debugf("triggered by restore inputs")
					inputs, _ = o.scan(inputs, next, ctx)
					ctx.PutState(WINDOW_INPUTS_KEY, inputs)
					ctx.PutState(TRIGGER_TIME_KEY, o.triggerTime)
				}
			}
		}
	}

	for {
		select {
		// process incoming item
		case item, opened := <-o.input:
			processed := false
			if item, processed = o.preprocess(item); processed {
				break
			}
			o.statManager.IncTotalRecordsIn()
			o.statManager.ProcessTimeStart()
			if !opened {
				o.statManager.IncTotalExceptions()
				break
			}
			switch d := item.(type) {
			case error:
				o.Broadcast(d)
				o.statManager.IncTotalExceptions()
			case *xsql.Tuple:
				log.Debugf("Event window receive tuple %s", d.Message)
				inputs = append(inputs, d)
				switch o.window.Type {
				case ast.NOT_WINDOW:
					inputs, _ = o.scan(inputs, d.Timestamp, ctx)
				case ast.SLIDING_WINDOW:
					inputs, _ = o.scan(inputs, d.Timestamp, ctx)
				case ast.SESSION_WINDOW:
					if timeoutTicker != nil {
						timeoutTicker.Stop()
						timeoutTicker.Reset(time.Duration(o.window.Interval) * time.Millisecond)
					} else {
						timeoutTicker = conf.GetTimer(o.window.Interval)
						timeout = timeoutTicker.C
						o.triggerTime = d.Timestamp
						ctx.PutState(TRIGGER_TIME_KEY, o.triggerTime)
						log.Debugf("Session window set start time %d", o.triggerTime)
					}
				case ast.COUNT_WINDOW:
					o.msgCount++
					log.Debugf(fmt.Sprintf("msgCount: %d", o.msgCount))
					if o.msgCount%o.window.Interval != 0 {
						continue
					} else {
						o.msgCount = 0
					}

					if tl, er := NewTupleList(inputs, o.window.Length); er != nil {
						log.Error(fmt.Sprintf("Found error when trying to "))
						errCh <- er
					} else {
						log.Debugf(fmt.Sprintf("It has %d of count window.", tl.count()))
						for tl.hasMoreCountWindow() {
							tsets := tl.nextCountWindow()
							log.Debugf("Sent: %v", tsets)
							//blocking if one of the channel is full
							o.Broadcast(tsets)
							o.statManager.IncTotalRecordsOut()
						}
						inputs = tl.getRestTuples()
					}
				}
				o.statManager.ProcessTimeEnd()
				o.statManager.SetBufferLength(int64(len(o.input)))
				ctx.PutState(WINDOW_INPUTS_KEY, inputs)
				ctx.PutState(MSG_COUNT_KEY, o.msgCount)
			default:
				o.Broadcast(fmt.Errorf("run Window error: expect xsql.Tuple type but got %[1]T(%[1]v)", d))
				o.statManager.IncTotalExceptions()
			}
		case now := <-c:
			n := cast.TimeToUnixMilli(now)
			if o.window.Type == ast.SESSION_WINDOW {
				log.Debugf("session window update trigger time %d with %d inputs", n, len(inputs))
				if len(inputs) == 0 || n-int64(o.window.Length) < inputs[0].Timestamp {
					if len(inputs) > 0 {
						log.Debugf("session window last trigger time %d < first tuple %d", n-int64(o.window.Length), inputs[0].Timestamp)
					}
					break
				}
			}
			if len(inputs) > 0 {
				o.statManager.ProcessTimeStart()
				log.Debugf("triggered by ticker at %d", n)
				inputs, _ = o.scan(inputs, n, ctx)
				o.statManager.ProcessTimeEnd()
				ctx.PutState(WINDOW_INPUTS_KEY, inputs)
				ctx.PutState(TRIGGER_TIME_KEY, o.triggerTime)
			}
		case now := <-timeout:
			if len(inputs) > 0 {
				o.statManager.ProcessTimeStart()
				log.Debugf("triggered by timeout")
				inputs, _ = o.scan(inputs, cast.TimeToUnixMilli(now), ctx)
				//expire all inputs, so that when timer scan there is no item
				inputs = make([]*xsql.Tuple, 0)
				o.statManager.ProcessTimeEnd()
				ctx.PutState(WINDOW_INPUTS_KEY, inputs)
				ctx.PutState(TRIGGER_TIME_KEY, o.triggerTime)
				timeoutTicker = nil
			}
		// is cancelling
		case <-ctx.Done():
			log.Infoln("Cancelling window....")
			if o.ticker != nil {
				o.ticker.Stop()
			}
			return
		}
	}
}

type TupleList struct {
	tuples []*xsql.Tuple
	index  int //Current index
	size   int //The size for count window
}

func NewTupleList(tuples []*xsql.Tuple, windowSize int) (TupleList, error) {
	if windowSize <= 0 {
		return TupleList{}, fmt.Errorf("Window size should not be less than zero.")
	} else if tuples == nil || len(tuples) == 0 {
		return TupleList{}, fmt.Errorf("The tuples should not be nil or empty.")
	}
	tl := TupleList{tuples: tuples, size: windowSize}
	return tl, nil
}

func (tl *TupleList) hasMoreCountWindow() bool {
	if len(tl.tuples) < tl.size {
		return false
	}
	return tl.index == 0
}

func (tl *TupleList) count() int {
	if len(tl.tuples) < tl.size {
		return 0
	} else {
		return 1
	}
}

func (tl *TupleList) nextCountWindow() xsql.WindowTuplesSet {
	results := xsql.WindowTuplesSet{
		Content: make([]xsql.WindowTuples, 0),
	}
	var subT []*xsql.Tuple
	subT = tl.tuples[len(tl.tuples)-tl.size : len(tl.tuples)]
	for _, tuple := range subT {
		results = results.AddTuple(tuple)
	}
	tl.index = tl.index + 1
	return results
}

func (tl *TupleList) getRestTuples() []*xsql.Tuple {
	if len(tl.tuples) < tl.size {
		return tl.tuples
	}
	return tl.tuples[len(tl.tuples)-tl.size+1:]
}

func (o *WindowOperator) scan(inputs []*xsql.Tuple, triggerTime int64, ctx api.StreamContext) ([]*xsql.Tuple, bool) {
	log := ctx.GetLogger()
	log.Debugf("window %s triggered at %s(%d)", o.name, time.Unix(triggerTime/1000, triggerTime%1000), triggerTime)
	var delta int64
	if o.window.Type == ast.HOPPING_WINDOW || o.window.Type == ast.SLIDING_WINDOW {
		delta = o.calDelta(triggerTime, delta, log)
	}
	results := xsql.WindowTuplesSet{
		Content: make([]xsql.WindowTuples, 0),
		WindowRange: &xsql.WindowRange{
			WindowEnd: triggerTime,
		},
	}
	i := 0
	//Sync table
	for _, tuple := range inputs {
		if o.window.Type == ast.HOPPING_WINDOW || o.window.Type == ast.SLIDING_WINDOW {
			diff := triggerTime - tuple.Timestamp
			if diff > int64(o.window.Length)+delta {
				log.Debugf("diff: %d, length: %d, delta: %d", diff, o.window.Length, delta)
				log.Debugf("tuple %s emitted at %d expired", tuple, tuple.Timestamp)
				//Expired tuple, remove it by not adding back to inputs
				continue
			}
			//Added back all inputs for non expired events
			inputs[i] = tuple
			i++
		} else if tuple.Timestamp > triggerTime {
			//Only added back early arrived events
			inputs[i] = tuple
			i++
		}
		if tuple.Timestamp <= triggerTime {
			results = results.AddTuple(tuple)
		}
	}
	triggered := false
	if len(results.Content) > 0 {
		switch o.window.Type {
		case ast.TUMBLING_WINDOW, ast.SESSION_WINDOW:
			results.WindowStart = o.triggerTime
		case ast.HOPPING_WINDOW:
			results.WindowStart = o.triggerTime - int64(o.window.Interval)
		case ast.SLIDING_WINDOW:
			results.WindowStart = triggerTime - int64(o.window.Length)
		}
		log.Debugf("window %s triggered for %d tuples", o.name, len(inputs))
		if o.isEventTime {
			results.Sort()
		}
		log.Debugf("Sent: %v", results)
		//blocking if one of the channel is full
		o.Broadcast(results)
		triggered = true
		o.triggerTime = triggerTime
		o.statManager.IncTotalRecordsOut()
		log.Debugf("done scan")
	}

	return inputs[:i], triggered
}

func (o *WindowOperator) calDelta(triggerTime int64, delta int64, log api.Logger) int64 {
	lastTriggerTime := o.triggerTime
	if lastTriggerTime <= 0 {
		delta = math.MaxInt16 //max int, all events for the initial window
	} else {
		if !o.isEventTime && o.window.Interval > 0 {
			delta = triggerTime - lastTriggerTime - int64(o.window.Interval)
			if delta > 100 {
				log.Warnf("Possible long computation in window; Previous eviction time: %d, current eviction time: %d", lastTriggerTime, triggerTime)
			}
		} else {
			delta = 0
		}
	}
	return delta
}

func (o *WindowOperator) GetMetrics() [][]interface{} {
	if o.statManager != nil {
		return [][]interface{}{
			o.statManager.GetMetrics(),
		}
	} else {
		return nil
	}
}
