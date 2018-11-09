package main

import (
	"log"
	"os"
	"text/template"
)

var queries = []string{
	"QueueLeaf",
	"AddSequencedLeaf",
	"GetInclusionProof",
	"GetInclusionProofByHash",
	"GetConsistencyProof",
	"GetLatestSignedLogRoot",
	"GetSequencedLeafCount",
	"GetEntryAndProof",
	"InitLog",
	"QueueLeaves",
	"AddSequencedLeaves",
	"GetLeavesByIndex",
	"GetLeavesByRange",
	"GetLeavesByHash",
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	tmpl, err := template.ParseFiles("./../admin/gen-server-client/client.go.txt")
	if err != nil {
		log.Fatal(err)
	}
	fh, err := os.Create("./client.go")
	if err != nil {
		log.Fatal(err)
	} else if err := tmpl.Execute(fh, queries); err != nil {
		log.Fatal(err)
	}
	fh.Close()
}
