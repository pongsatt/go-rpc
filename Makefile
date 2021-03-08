test:
	go test
testreport:
	go test -coverprofile=coverage.out
	go tool cover -html=coverage.out 
