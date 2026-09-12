package configurewebsite

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/desain-gratis/common/lib/notifier"
	configurewebsite "github.com/desain-gratis/deployd/internal/src/raft-app/configure-website"
	"github.com/desain-gratis/deployd/internal/src/utility"
	"github.com/desain-gratis/deployd/src/entity"
)

type localWorker struct {
	nginxUnitConfigAddress string

	dependencies *Dependencies
	host         *entity.Host

	// todo: can add lock
	deploymentJobPool map[string]*configureJob

	// todo: use input channel to queue job based on namespace & service
	// map[{ns, service}]chan <- msg

	// Also, retry mechanism / timeout handling, and resilliency should be coded
	// eg. after job is QUEUED, if local worker already setting up in memory & ready, local worker need to update to the raft back (we already implement)
	// what we havent is, if the local worker does not report back for various reason (only if they're expected to reply back)

	// controller level log
	log *slog.Logger

	// TODO: use worker pool B-)

	// TODO: later, after have many job types,
	// consider this localWorker can contain multiple types of job or just single

	// other types of job can be put here
}

// Local host state
type HostDeploymentJobState struct {
	ConfigurationStatus  entity.HostConfigurationStatus  `json:"configuration_status"`
	RestartServiceStatus entity.HostRestartServiceStatus `json:"restart_service_status"`
}

func (w *localWorker) configureWebsite(out notifier.Topic, jobDefinition *entity.JobConfigureWebsite) error {
	log := w.log

	if jobDefinition == nil {
		log.Warn("empty job definition") // TODO: implement
		return fmt.Errorf("empty job definition")
	}

	var err error

	ctx := context.Background()

	defer func() {
		updateTime := time.Now()
		result := entity.HostUpdateResult{
			Status:  entity.JobConfigureWebsiteStatusSuccess,
			Message: "success loh yaa",
			HostID:  w.host.Host,
			JobID:   jobDefinition.Id,
			Error:   nil,
			Time:    &updateTime,
		}
		if err != nil {
			errMsg := err.Error()
			result = entity.HostUpdateResult{
				Status:  entity.JobConfigureWebsiteStatusFailed,
				Message: "failed loh yaa :(",
				HostID:  w.host.Host,
				JobID:   jobDefinition.Id,
				Error:   &errMsg,
				Time:    &updateTime,
			}
		}
		_, err = w.dependencies.RaftConfigureWebsite.GenericUpdate(
			ctx,
			configurewebsite.Command_Host_ConfigureWebsiteResponse,
			result,
		)
	}()

	websiteDir := "/var/www/" + jobDefinition.Request.WebsiteHostAddress

	err = w.downloadWebsite(ctx, websiteDir, jobDefinition)
	if err != nil { // TODO: make proper
		return fmt.Errorf("failed to download website: %w", err)
	}

	err = w.updateNginxUnit(websiteDir)
	if err != nil { // TODO: make proper
		return fmt.Errorf("failed updating nginx unit config: %w", err)
	}

	return nil
}

func (w *localWorker) downloadWebsite(ctx context.Context, websiteDir string, jobDefinition *entity.JobConfigureWebsite) error {
	// create temporariy dir
	downloadLocation := "/tmp/configure-website/" + jobDefinition.Request.RepositoryID + "/artifact_" + jobDefinition.Request.BuildID + ".tar.gz"

	err := ensureDir("/tmp/configure-website/" + jobDefinition.Request.RepositoryID)
	if err != nil {
		return fmt.Errorf("error while ensureing dir file %w", err)
	}

	f, err := os.OpenFile(downloadLocation, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("error while opening tmp archive file %w", err)
	}
	defer f.Close()

	osarch := fmt.Sprintf("%s/%s", w.host.OS, w.host.Architecture)

	buildArtifact, meta, err := w.dependencies.BuildArtifactUsecase.GetAttachment(
		ctx,
		jobDefinition.Ns,
		[]string{jobDefinition.Request.RepositoryID, jobDefinition.Request.BuildID},
		osarch, // attachment can have one to many, so we're restricting to one
	)
	if err != nil {
		return fmt.Errorf("error while getting build artifact attachment: for ns=%v repository id=%v build id=%v os/arch=%v : %w",
			jobDefinition.Ns,
			jobDefinition.Request.RepositoryID,
			jobDefinition.Request.BuildID,
			osarch,
			err,
		)
	}
	defer buildArtifact.Close()

	// Download
	total, err := Copy(ctx, f, buildArtifact)
	if err != nil {
		return fmt.Errorf("error while writing artifact file %w", err)
	}

	if meta.ContentSize != uint64(total) {
		// maybe check hash
		return fmt.Errorf("download file size not matching! expected %v got %v", meta.ContentSize, total)
	}

	err = utility.ExtractTarGzStrip(downloadLocation, websiteDir)
	if err != nil {
		return fmt.Errorf("error while extracting artifact file: %w", err)
	}

	return nil
}

func (w *localWorker) updateNginxUnit(targetDir string) error {

	// 1. Define the path to NGINX Unit's control socket
	// Common paths: "/var/run/unit/control.sock" or "/var/run/control.unit.sock"
	socketPath := "/var/run/unit/control.sock"

	// 2. Define your NGINX Unit configuration JSON
	configJSON := []byte(`{
		"listeners": {
			"` + w.host.InternalAddress + `:80": {
				"pass": "routes"
			}
		},
		"routes": [
			{
				"action": {
					"share": "` + targetDir + `$uri",
					"fallback": {
                    	"share": "/var/www/html/404.html",
                    	"response": {
                        	"status": 404,
                        	"headers": {
                            	"Content-Type": "text/html"
                        	}
                    	}
                	}
				}
			}
		]
	}`)

	// 3. Create a custom HTTP client that dials the Unix socket
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
	}

	// 4. Build the PUT request targeting Unit's local configuration endpoint
	// The host part of the URL ("http://localhost") is ignored by the Unix dialer
	req, err := http.NewRequest(http.MethodPut, "http://localhost/config/", bytes.NewReader(configJSON))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// 5. Execute the request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error connecting to Unit socket: %v\n", err)
		fmt.Println("Tip: Make sure Go is running with sudo permissions to read/write to the socket file.")
		return err
	}
	defer resp.Body.Close()

	// 6. Read and print the API response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %v\n", err)
		return err
	}

	fmt.Printf("HTTP Status: %s\n", resp.Status)
	fmt.Printf("Response Body: %s\n", string(body))

	return nil
}

// GPTmaxxing
func Copy(ctx context.Context, dst io.Writer, src io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	total := 0
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return 0, werr
			}
		}
		total = total + n
		if err != nil {
			if err == io.EOF {
				return total, nil
			}
			return 0, err
		}
	}
}

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}
