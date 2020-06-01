OUTFILE=smtpBrute.exe

.PHONY: main fmt vet clean check compile run

main: compile

fmt:
	go fmt ./...

vet:
	go vet ./...

check: fmt vet

clean:
	go clean
	rm -f $(OUTFILE)

compile: clean check
	go build -ldflags "-s -w" -trimpath -o $(OUTFILE) .

run: compile
	./$(OUTFILE) -t localhost
