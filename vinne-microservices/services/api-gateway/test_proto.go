package main

import (
	"fmt"
	ticketv1 "github.com/randco/randco-microservices/proto/ticket/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	ticket := &ticketv1.Ticket{
		Id:           "test-id-123",
		SerialNumber: "TKT-123",
		GameCode:     "FRINR",
		GameName:     "FridayNoonRush",
		TotalAmount:  35000,
	}

	// Test with UseProtoNames: false
	marshaler := protojson.MarshalOptions{
		UseProtoNames:   false,
		EmitUnpopulated: true,
	}
	jsonBytes, _ := marshaler.Marshal(ticket)
	fmt.Println("UseProtoNames: false")
	fmt.Println(string(jsonBytes))

	// Test with UseProtoNames: true
	marshaler2 := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}
	jsonBytes2, _ := marshaler2.Marshal(ticket)
	fmt.Println("\nUseProtoNames: true")
	fmt.Println(string(jsonBytes2))
}
