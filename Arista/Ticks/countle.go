package main

import (
	"log"
	"os"
	"time"

	"github.com/influxdata/kapacitor/udf"
	"github.com/influxdata/kapacitor/udf/agent"
)

// This UDF filters the incoming points
//   Each point is expected to have an int field named 'count'
//   Every point whose 'count' fields's value is LESS-THAN-EQUAL
//   the configured "threshold" is returned
//   No change is done to the incoming point
//
//   NOTE: if the incoming point doesn't have an int field named 'count'
//   then the point is not returned from this UDF
type countLeHandler struct {
	threshold     int64
	roundDuration int64
	points        []*udf.Point
	agent         *agent.Agent
}

func newCountLeHandler(agent *agent.Agent) *countLeHandler {
	return &countLeHandler{agent: agent, threshold: 0, roundDuration: 0, points: nil}
}

func (o *countLeHandler) reset() {
	o.points = nil
}

func (o *countLeHandler) addPoint(p *udf.Point) {
	o.points = append(o.points, p)
}

// Return the InfoResponse. Describing the properties of this UDF agent.
func (*countLeHandler) Info() (*udf.InfoResponse, error) {
	info := &udf.InfoResponse{
		Wants:    udf.EdgeType_BATCH,
		Provides: udf.EdgeType_BATCH,
		Options: map[string]*udf.OptionInfo{
			"threshold":     {ValueTypes: []udf.ValueType{udf.ValueType_INT}},
			"roundDuration": {ValueTypes: []udf.ValueType{udf.ValueType_DURATION}},
		},
	}
	return info, nil
}

// Initialze the handler based on the provided options.
func (o *countLeHandler) Init(r *udf.InitRequest) (*udf.InitResponse, error) {
	init := &udf.InitResponse{
		Success: true,
		Error:   "",
	}

	for _, opt := range r.Options {
		switch opt.Name {
		case "threshold":
			o.threshold = opt.Values[0].Value.(*udf.OptionValue_IntValue).IntValue
		case "roundDuration":
			o.roundDuration = opt.Values[0].Value.(*udf.OptionValue_DurationValue).DurationValue
		}
	}

	if o.threshold < 0 {
		init.Success = false
		init.Error = "threshold must be non-negative"
	}

	return init, nil
}

// Create a snapshot of the running state of the process.
func (o *countLeHandler) Snaphost() (*udf.SnapshotResponse, error) {
	// this UDF does not need a snapshot/restore implementation since there
	// is no state maintained across each UDF invocation
	return &udf.SnapshotResponse{}, nil
}

// Restore a previous snapshot.
func (o *countLeHandler) Restore(req *udf.RestoreRequest) (*udf.RestoreResponse, error) {
	// this UDF does not need a snapshot/restore implementation since there
	// is no state maintained across each UDF invocation
	return &udf.RestoreResponse{
		Success: true,
	}, nil
}

// Start working with the next batch
func (o *countLeHandler) BeginBatch(begin *udf.BeginBatch) error {
	o.reset()

	o.agent.Responses <- &udf.Response{
		Message: &udf.Response_Begin{
			Begin: begin,
		},
	}
	return nil
}

func (o *countLeHandler) Point(p *udf.Point) error {

	if value, ok := p.FieldsDouble["count"]; ok {
		if int64(value) <= o.threshold {
			// Rounding off the point to duration of the UDF run
			now := time.Now().UTC().UnixNano()
			p.Time = now - (now % o.roundDuration)
			o.addPoint(p)
		}
	}

	return nil
}

func (o *countLeHandler) EndBatch(end *udf.EndBatch) error {
	for _, older := range o.points {
		log.Println("COUNTLE %+v", older)
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
func (o *countLeHandler) Stop() {
	log.Println("Stopping countLeHandler agent")
	close(o.agent.Responses)
}

func main() {
	a := agent.New(os.Stdin, os.Stdout)
	h := newCountLeHandler(a)
	a.Handler = h

	log.Println("Starting countLeHandler agent")
	a.Start()
	err := a.Wait()
	if err != nil {
		log.Fatal(err)
	}
}
