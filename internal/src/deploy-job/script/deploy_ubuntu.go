package script

import (
	"context"
	"os"
)

type DeploymentFiles struct {
	ConfigFolder *os.File
	BuildFolder  *os.File
	// secret later should be on the deployd env itself
	SystemdFile *os.File
}

func UbuntuOpen(ctx context.Context, serviceName string, buildVersion, envVersion, secretVersion uint64) {

}
