module github.com/go-anyway/framework-hotreload

go 1.25.4

require (
	github.com/go-anyway/framework-log v1.0.0
)

replace (
	github.com/go-anyway/framework-log => ../core/log
)
