package function

import (
	flow "github.com/s8sg/faas-flow"
	consulStateStore "github.com/s8sg/faas-flow-consul-statestore"
	"os"
	// minioDataStore "github.com/s8sg/faas-flow-minio-datastore"
	// "log"
)

// Define provide definiton of the saga
func Define(workflow *flow.Workflow, context *flow.Context) (err error) {

	sagaDag := flow.CreateDag()
	// as we override sagaDag with branch dags,
	// we should set the main execution dag here
	workflow.ExecuteDag(sagaDag)

	// Step 1:
	// generate a booking Id and add to request context
	sagaDag.AddFunction("generate-booking-id", "generate-id")
	sagaDag.AddModifier("generate-id", func(data []byte) ([]byte, error) {
		context.Set("booking-id", string(data))
		return data, nil
	})

	// Step 2:
	// Book Flight
	sagaDag.AddFunction("flight", "flight-booking",
		flow.Query("request", "book"),
		//flow.Query("booking-id", context.GetString("booking-id")),
	)
	branchdags := sagaDag.AddConditionalBranch("check-flightbooking-status",
		// the possible condition
		[]string{"success", "failure"},
		// function that determine the status
		func(response []byte) []string {
			result := string(response)
			return []string{result}
		},
	)
	sagaDag.AddEdge("generate-id", "flight")
	sagaDag.AddEdge("flight", "check-flightbooking-status", flow.Execution)

	// Handle flight booking failure
	failureDag := branchdags["failure"]
	failureDag.AddFunction("inform-failure", "inform-status",
		flow.Query("status", "failure"),
	)

	// Step 3:
	// Book hotel
	sagaDag = branchdags["success"]
	sagaDag.AddFunction("hotel", "hotel-booking",
		flow.Query("request", "book"),
		//flow.Query("booking-id", context.GetString("booking-id")),
	)
	branchdags = sagaDag.AddConditionalBranch("check-hotelbooking-status",
		// the possible confdition
		[]string{"success", "failure"},
		// function that determine the status
		func(response []byte) []string {
			result := string(response)
			return []string{result}
		},
	)
	sagaDag.AddEdge("hotel", "check-hotelbooking-status", flow.Execution)

	// Handle hotel booking failure
	failureDag = branchdags["failure"]
	failureDag.AddFunction("flight-cancel", "flight-booking",
		flow.Query("request", "cancel"),
		//flow.Query("booking-id", context.GetString("booking-id")),
	)
	failureDag.AddFunction("inform-failure", "inform-status",
		flow.Query("status", "failure"),
	)
	failureDag.AddEdge("flight-cancel", "inform-failure", flow.Execution)

	// Step 4:
	// Book car
	sagaDag = branchdags["success"]
	sagaDag.AddFunction("car", "rent-car")
	branchdags = sagaDag.AddConditionalBranch("check-carbooking-status",
		[]string{"success", "failure"},
		// function that determine the status
		func(response []byte) []string {
			result := string(response)
			return []string{result}
		},
	)
	sagaDag.AddEdge("car", "check-carbooking-status", flow.Execution)

	// handle car booking failure
	failureDag = branchdags["failure"]
	failureDag.AddFunction("hotel-cancel", "hotel-booking",
		flow.Query("request", "cancel"),
		//flow.Query("booking-id", context.GetString("booking-id")),
	)
	failureDag.AddFunction("flight-cancel", "flight-booking",
		flow.Query("request", "cancel"),
		//flow.Query("booking-id", context.GetString("booking-id")),
	)
	failureDag.AddFunction("inform-failure", "inform-status",
		flow.Query("status", "failure"),
	)
	failureDag.AddEdge("hotel-cancel", "flight-cancel", flow.Execution)
	failureDag.AddEdge("flight-cancel", "inform-failure", flow.Execution)

	// Step 5:
	// inform success
	sagaDag = branchdags["success"]
	sagaDag.AddFunction("inform-success", "inform-status")

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
