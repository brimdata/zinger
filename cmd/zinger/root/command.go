package root

import (
	"flag"
	"log"

	"github.com/mccanne/charm"
)

// These variables are populated via the Go linker.
var (
	Version   = "unknown"
	ZqVersion = "unknown"
)

var Zinger = &charm.Spec{
	Name:  "zinger",
	Usage: "zinger [global options] command [options] [arguments...]",
	Short: "use zinger to receive, store, and transform zeek logs",
	Long: `
Zinger interconnects zeek and Kafka/Acvro using the Kafka Schema Registery.

`,
	New: New,
}

type Command struct {
	charm.Command
}

func init() {
	Zinger.Add(charm.Help)
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{}
	log.SetPrefix("zinger")
	return c, nil
}

func (c *Command) Run(args []string) error {
	return Zinger.Exec(c, []string{"help"})
}
