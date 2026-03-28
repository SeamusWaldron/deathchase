CGO_CFLAGS = -Wno-undef-prefix

.PHONY: build run clean

build:
	CGO_CFLAGS="$(CGO_CFLAGS)" go build -o deathchase .

run:
	CGO_CFLAGS="$(CGO_CFLAGS)" go run .

clean:
	rm -f deathchase
