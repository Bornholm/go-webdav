CPU_COUNT := $(shell nproc --all 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 1)
CPU_QUARTER := $(shell echo $$((($(CPU_COUNT) + 3) / 4)))

benchmark:
	go test -bench=. -cpu $(CPU_QUARTER) -benchmem ./...

include misc/**/*.mk