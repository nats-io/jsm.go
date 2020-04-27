package api

import (
	"fmt"
	"sort"
	"strings"
)

func ExampleValidateStruct() {
	event := StreamConfig{}

	ok, errs := ValidateStruct(event, event.SchemaType())
	if !ok {
		sort.Strings(errs)
		fmt.Println("Event Validation Failed:")
		fmt.Printf("   %s\n", strings.Join(errs, "\n   "))
	}

	// Output:
	// Event Validation Failed:
	//    num_replicas: Must be greater than or equal to 1
	//    retention: retention must be one of the following: "limits", "interest", "workqueue"
	//    storage: storage must be one of the following: "file", "memory"
}
