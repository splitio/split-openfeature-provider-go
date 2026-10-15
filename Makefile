.PHONY: test

# centralisedGoCheckDynamic runs this target and uploads coverage.out.
test:
	go test -count=1 -coverprofile=coverage.out -covermode=atomic ./...
