

fmt:
	find . -type f -name '*.go' ! -path './vendor/*' ! -path './pb/*' ! -path './proto_gen/*' ! -path './idl/*' -exec \
	  goimports -l -w -local "github.com/ahfuzhang/spscqueue" {} +
