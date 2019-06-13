package function

import (
	"fmt"
	flow "github.com/s8sg/faas-flow"
	consulStateStore "github.com/s8sg/faas-flow-consul-statestore"
	"os"
	// minioDataStore "github.com/s8sg/faas-flow-minio-datastore"
	// "log"
)

func defineBookingDag(bookingDag *flow.DagFlow, context *flow.Context) {
	// Step 2:
	// Book Flight
	bookingDag.AddFunction("flight",
		"flight-booking",
		flow.Query("request", "book"),
		flow.Forwarder(func(_ []byte) []byte {
			data := []byte(fmt.Sprintf("id=%s", context.GetString("booking-id")))
			return data
		}),
	)
	branchdags := bookingDag.AddConditionalBranch("check-status",
		// the possible condition
		[]string{"success", "failure"},
		// function that determine the status
		func(response []byte) []string {
			return []string{"success"}
		},
		flow.ExecutionBranch,
	)

	bookingDag.AddEdge("flight", "check-status", flow.Execution)

	// Handle Flight booking failure
	failureDag := branchdags["failure"]
	failureDag.AddModifier("update-context", func(data []byte) ([]byte, error) {
		context.Set("status", "failure")
		return data, nil
	})

	// Step 3:
	// Book hotel
	bookingDag = branchdags["success"]
	bookingDag.AddFunction("hotel",
		"hotel-booking",
		flow.Query("request", "book"),
		flow.Forwarder(func(_ []byte) []byte {
			data := []byte(fmt.Sprintf("id=%s", context.GetString("booking-id")))
			return data
		}),
	)
	branchdags = bookingDag.AddConditionalBranch("check-status",
		// the possible confdition
		[]string{"success", "failure"},
		// function that determine the status
		func(response []byte) []string {
			context.Set("status", "success")
			return []string{"success"}
		},
		flow.ExecutionBranch,
	)
	bookingDag.AddEdge("hotel", "check-status", flow.Execution)

	// Handle hotel booking failure
	failureDag = branchdags["failure"]
	failureDag.AddModifier("update-context", func(data []byte) ([]byte, error) {
		context.Set("status", "failure")
		return data, nil
	})
	failureDag.AddFunction("flight-cancel",
		"flight-booking",
		flow.Query("request", "cancel"),
		flow.Forwarder(func(_ []byte) []byte {
			data := []byte(fmt.Sprintf("id=%s", context.GetString("booking-id")))
			return data
		}),
	)
	failureDag.AddEdge("update-context", "flight-cancel", flow.Execution)

	// Step 4:
	// Book car
	bookingDag = branchdags["success"]
	bookingDag.AddFunction("car",
		"rent-car",
		flow.Query("request", "book"),
		flow.Forwarder(func(_ []byte) []byte {
			data := []byte(fmt.Sprintf("id=%s", context.GetString("booking-id")))
			return data
		}),
	)

	branchdags = bookingDag.AddConditionalBranch("check-status",
		[]string{"success", "failure"},
		// function that determine the status
		func(response []byte) []string {
			return []string{"failure"}
		},
		flow.ExecutionBranch,
	)
	bookingDag.AddEdge("car", "check-status", flow.Execution)

	// handle car booking failure
	failureDag = branchdags["failure"]
	failureDag.AddModifier("update-context", func(data []byte) ([]byte, error) {
		context.Set("status", "failure")
		return data, nil
	})
	failureDag.AddFunction("hotel-cancel", "hotel-booking",
		flow.Query("request", "cancel"),
		flow.Forwarder(func(_ []byte) []byte {
			data := []byte(fmt.Sprintf("id=%s", context.GetString("booking-id")))
			return data
		}),
	)
	failureDag.AddFunction("flight-cancel", "flight-booking",
		flow.Query("request", "cancel"),
		flow.Forwarder(func(_ []byte) []byte {
			data := []byte(fmt.Sprintf("id=%s", context.GetString("booking-id")))
			return data
		}),
	)
	failureDag.AddEdge("update-context", "hotel-cancel", flow.Execution)
	failureDag.AddEdge("hotel-cancel", "flight-cancel", flow.Execution)

	// Step 5
	// Update status to success
	bookingDag = branchdags["success"]
	bookingDag.AddModifier("update-context", func(data []byte) ([]byte, error) {
		context.Set("status", "success")
		return data, nil
	})
}

// Define provide definiton of the saga
func Define(workflow *flow.Workflow, context *flow.Context) (err error) {

	sagaDag := flow.CreateDag()
	// as we override sagaDag with branch dags,
	// we should set the main execution dag here
	workflow.ExecuteDag(sagaDag)

	// Step 1:
	// generate a booking Id and add to request context
	sagaDag.AddFunction("generate-id", "generate-id")
	sagaDag.AddModifier("generate-id", func(data []byte) ([]byte, error) {
		context.Set("booking-id", string(data))
		return data, nil
	})
	// Create a subdag to manage boooking
	bookingDag := sagaDag.AddSubDag("booking-handler")
	sagaDag.AddFunction("inform-status",
		"inform-status",
		flow.Forwarder(func(_ []byte) []byte {
			data := []byte(fmt.Sprintf("id=%s, status=%s",
				context.GetString("booking-id"),
				context.GetString("status")))
			return data
		}),
	)

	sagaDag.AddEdge("generate-id", "booking-handler", flow.Execution)
	sagaDag.AddEdge("booking-handler", "inform-status", flow.Execution)

	// define the booking dag
	defineBookingDag(bookingDag, context)

	return
}

// DefineStateStore provides the override of the default StateStore
func DefineStateStore() (flow.StateStore, error) {
	consulss, err := consulStateStore.GetConsulStateStore(
		os.Getenv("consul_url"),
		os.Getenv("consul_dc"),
	)
	if err != nil {
		return nil, err
	}
	return consulss, nil
	return nil, nil
}

// ProvideDataStore provides the override of the default DataStore
func DefineDataStore() (flow.DataStore, error) {
	return nil, nil
}
