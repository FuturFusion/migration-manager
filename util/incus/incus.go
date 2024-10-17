package incus

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/cliconfig"

	"github.com/FuturFusion/migration-manager/util"
	"github.com/FuturFusion/migration-manager/util/progress"
	"github.com/FuturFusion/migration-manager/util/revert"
)

type IncusClient struct {
	client incus.InstanceServer
	ctx    context.Context
}

func NewIncusClient(ctx context.Context, incusRemoteName string) (*IncusClient, error) {
	var incusServer incus.InstanceServer
	var err error

	if incusRemoteName == "" {
		incusServer, err = connectLocal()
	} else {
		incusServer, err = connectRemote(incusRemoteName)
	}

	if err != nil {
		return nil, err
	}

	return &IncusClient{
		client: incusServer,
		ctx:    ctx,
	}, nil
}

func connectLocal() (incus.InstanceServer, error) {
	args := incus.ConnectionArgs{}
	args.UserAgent = fmt.Sprintf("LXC-MIGRATE %s", "1.2.3") // FIXME was pulling in internal version string from Incus

	return incus.ConnectIncusUnix("", &args)
}

func connectRemote(remoteName string) (incus.InstanceServer, error) {
	incusConfig, err := cliconfig.LoadConfig(path.Join(os.Getenv("HOME"), ".config", "incus", "config.yml"))
	if err != nil {
		return nil, err
	}

	return incusConfig.GetInstanceServer(remoteName)
}

func (c *IncusClient) CreateInstance(instanceArgs api.InstancesPost, nics []util.NICInfo) error {
        revert := revert.New()
        defer revert.Fail()

        // Create the instance
        op, err := c.client.CreateInstance(instanceArgs)
        if err != nil {
                return err
        }

        revert.Add(func() {
		_, _ = c.client.DeleteInstance(instanceArgs.Name)
        })

	for _, nic := range nics {
		// FIXME actually add to instance
		fmt.Printf("    Adding NIC for network %q with MAC %s\n", nic.Network, nic.Hwaddr)
	}

	prog := progress.ProgressRenderer{Format: "Transferring instance: %s"}
	_, err = op.AddHandler(prog.UpdateOp)
	if err != nil {
		prog.Done("")
		return err
	}

	err = op.Wait() // Fails due to missing rootfs logic
	if err != nil {
		return err
	}

	prog.Done(fmt.Sprintf("Instance %s successfully created", instanceArgs.Name))
	revert.Success()

	return nil
}
