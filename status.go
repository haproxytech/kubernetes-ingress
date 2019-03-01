package main

type Status string

const (
	ADDED    Status = "ADDED"
	MODIFIED Status = "MODIFIED"
	DELETED  Status = "DELETED"
	ERROR    Status = "ERROR"
	EMPTY    Status = ""
)
