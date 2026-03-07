package deployjob

import (
	"fmt"
	"os"
	"strconv"
)

func (c *configureHost) optConfigureRouting() error {
	log := c.log
	ctx := c.ctx

	// TODO: currently only for cloudfalre / refactor
	cloudflaredPath := "/usr/local/bin/cloudflared"
	_, err := os.Stat(cloudflaredPath)
	if err != nil {
		// maybe ignore (eg for docker)
		return fmt.Errorf("cloudflared binary not found on default path: %s", cloudflaredPath)
	}

	// TODO: separate it into their own module later...
	log.Info("configuring cloudflared routing..")

	if err := ctx.Err(); err != nil {
		return err
	}

	// Make sure the required release version directory is there in the host

	err = func() error {
		unitName := fmt.Sprintf("cloudflared_%s_%s.service", c.Job.Request.Service.Ns, c.Job.Request.Service.Id)
		final := "/etc/systemd/system/" + unitName
		tmp := "/etc/systemd/system/" + unitName + ".tmp"

		routingConfigs, err := c.deploymentJob.dependencies.RoutingUsecase.Get(
			c.ctx, c.Job.Request.Ns, []string{c.Job.Request.Service.Id}, strconv.FormatUint(*c.Job.Request.RoutingVersion, 10))
		if err != nil {
			// or if not found
			return fmt.Errorf("error while downloading cloudflared config %w", err)
		}

		routingConfig := routingConfigs[0]

		if routingConfig.CloudflareConfig != nil {
			log.Info("writing cloudflared config")

			err := func() error {
				f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
				if err != nil {
					return fmt.Errorf("error while opening cloudflared file in %v %w", tmp, err)
				}
				defer f.Close()

				_, err = fmt.Fprintf(f, `[Unit]
Description=Cloudflare Tunnel for %s/%s (v%v)
After=network-online.target
Wants=network-online.target

[Service]
TimeoutStartSec=0
Type=notify
ExecStart=/usr/local/bin/cloudflared --no-autoupdate tunnel run --token %s
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
`, c.Job.Request.Service.Ns, c.Job.Request.Service.Id, routingConfig.Version, routingConfig.CloudflareConfig.TunnelToken)
				if err != nil {
					return fmt.Errorf("failed to write cloudflared systemd file: %v", err)
				}
				return nil
			}()
			if err != nil {
				return err
			}

			err = os.Rename(tmp, final)
			if err != nil {
				return fmt.Errorf("error while replacing cloudflared systemd definition  %v %w", final, err)
			}

			return nil
		}
		return nil
	}()
	if err != nil {
		return err
	}

	return nil
}
