package main

import (
	"log"
	"os"
	"time"

	"github.com/influxdata/kapacitor/udf"
	"github.com/influxdata/kapacitor/udf/agent"
)

// This UDF adds a field noupdates to the incoming points
//   noupdates=2 is the point is older than now() - period
//   noupdates=1 otherwise
//   The points with noupdates=2 have their timestamp
//   rounded to roundDuration (this is for clairty
//   when viewing points in Chronograf)
type withoutUpdatesHandler struct {
	period        int64
	roundDuration int64
	points        []*udf.Point
	agent         *agent.Agent
}

func newWithoutUpdatesHandler(agent *agent.Agent) *withoutUpdatesHandler {
	return &withoutUpdatesHandler{agent: agent, period: 0, roundDuration: 0, points: nil}
}

func (o *withoutUpdatesHandler) reset() {
	o.points = nil
}

func (o *withoutUpdatesHandler) addPoint(p *udf.Point) {
	o.points = append(o.points, p)
}

// Return the InfoResponse. Describing the properties of this UDF agent.
func (*withoutUpdatesHandler) Info() (*udf.InfoResponse, error) {
	info := &udf.InfoResponse{
		Wants:    udf.EdgeType_BATCH,
		Provides: udf.EdgeType_BATCH,
		Options: map[string]*udf.OptionInfo{
			"period":        {ValueTypes: []udf.ValueType{udf.ValueType_DURATION}},
			"roundDuration": {ValueTypes: []udf.ValueType{udf.ValueType_DURATION}},
		},
	}
	return info, nil
}

// Initialze the handler based on the provided options.
func (o *withoutUpdatesHandler) Init(r *udf.InitRequest) (*udf.InitResponse, error) {
	init := &udf.InitResponse{
		Success: true,
		Error:   "",
	}

	for _, opt := range r.Options {
		switch opt.Name {
		case "period":
			o.period = opt.Values[0].Value.(*udf.OptionValue_DurationValue).DurationValue
		case "roundDuration":
			o.roundDuration = opt.Values[0].Value.(*udf.OptionValue_DurationValue).DurationValue
		}
	}

	if o.period == 0 {
		init.Success = false
		init.Error = "period must be a non-zero duration"
	}

	return init, nil
}

// Create a snapshot of the running state of the process.
func (o *withoutUpdatesHandler) Snaphost() (*udf.SnapshotResponse, error) {
	// this UDF does not need a snapshot/restore implementation since there
	// is no state maintained across each UDF invocation
	return &udf.SnapshotResponse{}, nil
}

// Restore a previous snapshot.
func (o *withoutUpdatesHandler) Restore(req *udf.RestoreRequest) (*udf.RestoreResponse, error) {
	// this UDF does not need a snapshot/restore implementation since there
	// is no state maintained across each UDF invocation
	return &udf.RestoreResponse{
		Success: true,
	}, nil
}

// Start working with the next batch
func (o *withoutUpdatesHandler) BeginBatch(begin *udf.BeginBatch) error {
	o.reset()

	o.agent.Responses <- &udf.Response{
		Message: &udf.Response_Begin{
			Begin: begin,
		},
	}
	return nil
}

func (o *withoutUpdatesHandler) Point(p *udf.Point) error {
	if p.Time < time.Now().UTC().UnixNano()-o.period {
		//log.Println(p, time.Now().UTC().String())
		// Each point is operated on in isolation w.r.t other points
		// Without the second rounding, use of nanoseconds causes
		// new points generated from the same run of the UDF to look
		// a bit scattered in Chronograf. The second rounding, keeps
		// such scattering under limit (doesn't fully avoid it)

		// Rounding off the point to duration of the UDF run
		now := time.Now().UTC().UnixNano()
		p.Time = now - (now % o.roundDuration)
		if nil == p.FieldsInt {
			p.FieldsInt = map[string]int64{"noupdates": 2}
		} else {
			p.FieldsInt["noupdates"] = 2
		}
	} else {
		if nil == p.FieldsInt {
			p.FieldsInt = map[string]int64{"noupdates": 1}
		} else {
			p.FieldsInt["noupdates"] = 1
		}
	}
	o.addPoint(p)
	return nil
}

func (o *withoutUpdatesHandler) EndBatch(end *udf.EndBatch) error {
	for _, older := range o.points {
		// log.Println("%+v", older)
		o.agent.Responses <- &udf.Response{
			Message: &udf.Response_Point{
				Point: older,
			},
		}
	}

	// End batch
	o.agent.Responses <- &udf.Response{
		Message: &udf.Response_End{
			End: end,
		},
	}
	return nil
}

// Stop the handler gracefully.
func (o *withoutUpdatesHandler) Stop() {
	log.Println("Stopping withoutUpdatesHandler agent")
	close(o.agent.Responses)
}

func main() {
	a := agent.New(os.Stdin, os.Stdout)
	h := newWithoutUpdatesHandler(a)
	a.Handler = h

	log.Println("Starting withoutUpdatesHandler agent")
	a.Start()
	err := a.Wait()
	if err != nil {
		log.Fatal(err)
	}
}
