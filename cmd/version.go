package cmd

import (
	"flag"
	"fmt"
	"runtime"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

type VersionCmd struct{}

func NewVersionCmd() *VersionCmd {
	return &VersionCmd{}
}

func (c *VersionCmd) Name() string        { return "version" }
func (c *VersionCmd) Description() string  { return "显示版本信息" }
func (c *VersionCmd) SetFlags(fs *flag.FlagSet) {}

func (c *VersionCmd) Run(args []string) error {
	fmt.Printf("pvctl %s\n", Version)
	fmt.Printf("  commit:    %s\n", GitCommit)
	fmt.Printf("  built:     %s\n", BuildDate)
	fmt.Printf("  go:        %s\n", runtime.Version())
	fmt.Printf("  os/arch:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return nil
}
