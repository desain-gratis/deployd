package configurations

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
)

var serviceName = "deployd"

var (
	baseOpt     = "./demo/opt/" + serviceName
	baseEtc     = "./demo/etc/" + serviceName
	systemdPath = "./demo/etc/systemd/system/" + serviceName + ".service"
	tmpFile     = "./demo/tmp/" + serviceName + "/0"
)

func Configure(
	deploydAPI string,
	namespace string,
	service string,
	release uint64,
) {
	// binaryURL := "http://mb1:9600/artifactd/archive?repository=user-profile&build=e9e6f7c40392224d5ff2e280af5d6bf82b045fda&id=linux/amd64&data=true"

	serviceName = service

	err := ensureDirs()
	if err != nil {
		log.Fatalf("cannot ensure dirs: %v", err)
	}

	binaryURL := "http://mb1:9600/artifactd/archive?repository=user-profile&build=e9e6f7c40392224d5ff2e280af5d6bf82b045fda&id=linux/amd64&data=true"
	// secretURL := mustEnv("SECRET_URL")
	// deploydAuth := os.Getenv("DEPLOYD_AUTH")
	// envURL := mustEnv("ENV_URL")

	// ctx := context.Background()

	// serviceClient := mycontentapiclient.New[*entity.ServiceDefinition](http.DefaultClient, deploydAPI+"/deployd/service", nil)
	// serviceConfigs, errClient := serviceClient.Get(ctx, deploydAuth, namespace, nil, service)
	// if errClient != nil {
	// 	log.Err(errClient).Msgf("failed to get service definition for service: %v", service)
	// 	return
	// }
	// if len(serviceConfigs) == 0 {
	// 	log.Err(errClient).Msgf("empty serice definition for service: %v", service)
	// 	return
	// }

	// tmpFile := "./tmpdownload/" + serviceName + "/" + strconv.FormatUint(release, 10) + ".tar.gz"

	fmt.Println("Downloading artifact...")
	downloadFile(binaryURL, tmpFile)

	releaseDir := filepath.Join(baseOpt, "releases", strconv.FormatUint(release, 10))
	must(os.MkdirAll(releaseDir, 0755))

	fmt.Println("Extracting...")
	extractTarGzStrip(tmpFile, releaseDir)

	// must(os.Chmod(filepath.Join(releaseDir, serviceName), 0755))

	// fmt.Println("Fetching secret & env...")
	// must(os.MkdirAll(baseEtc, 0755))
	// downloadFile(secretURL, filepath.Join(baseEtc, "secret.yaml"))
	// downloadFile(envURL, filepath.Join(baseEtc, "overwrite.env"))

	secretPath := filepath.Join(baseEtc, "secret.yaml")
	envPath := filepath.Join(baseEtc, "overwrite.env")

	must(touchFile(secretPath))
	must(touchFile(envPath))

	// fmt.Println("Writing systemd unit...")
	must(writeSystemdUnit())

	// fmt.Println("Switching current symlink...")
	switchSymlink(filepath.Join(baseOpt, "current"), releaseDir)

	// fmt.Println("Reloading & restarting service...")
	// must(systemdReloadEnableRestart())

	fmt.Println("Done.")
}

func ensureDir(dir string) error {
	parent := filepath.Dir(dir)

	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("mkdir parent %s: %w", parent, err)
	}

	return nil
}

func ensureDirs() error {
	dirs := []string{
		baseOpt,
		filepath.Join(baseOpt, "releases"),
		baseEtc,
		"./demo/etc/systemd/system",
		tmpFile,
	}

	for _, d := range dirs {
		if err := ensureDir(d); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		panic(k + " missing")
	}
	return v
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func downloadFile(url, dest string) {
	// Ensure parent dir exists
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		panic(err)
	}

	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("X-Namespace", "*")
	resp, err := http.DefaultClient.Do(req)
	must(err)
	defer resp.Body.Close()

	f, err := os.Create(dest)
	must(err)
	defer f.Close()

	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), resp.Body)
	must(err)

	fmt.Printf("Downloaded %s sha256=%x\n", dest, h.Sum(nil))
}

func extractTarGz(src, dest string) {
	f, err := os.Open(src)
	must(err)
	defer f.Close()

	gz, err := gzip.NewReader(f)
	must(err)
	defer gz.Close()

	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		must(err)

		target := filepath.Join(dest, hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := ensureDir(target); err != nil {
				log.Printf("extract dir: %v", err)
				continue
			}

			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			if err := ensureDir(target); err != nil {
				log.Printf("extract reg: %v", err)
				continue
			}
			out, err := os.Create(target)
			must(err)
			io.Copy(out, tr)
			out.Close()
		}
	}
}

func extractTarGzStrip(src, dest string) {
	f, err := os.Open(src)
	must(err)
	defer f.Close()

	gz, err := gzip.NewReader(f)
	must(err)
	defer gz.Close()

	tr := tar.NewReader(gz)

	var prefix string // first folder name to strip

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		must(err)

		name := hdr.Name

		// Detect top-level folder once
		if prefix == "" {
			parts := strings.SplitN(name, "/", 2)
			if len(parts) > 1 {
				prefix = parts[0] + "/"
			}
		}

		// Strip prefix
		name = strings.TrimPrefix(name, prefix)
		if name == "" {
			continue
		}

		target := filepath.Join(dest, name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			must(os.MkdirAll(target, 0755))

		case tar.TypeReg:
			must(os.MkdirAll(filepath.Dir(target), 0755))

			out, err := os.Create(target)
			must(err)

			_, err = io.Copy(out, tr)
			out.Close()
			must(err)
		}
	}
}

func switchSymlink(link, target string) {
	tmp := link + ".tmp"
	os.Remove(tmp)
	must(os.Symlink(target, tmp))
	must(os.Rename(tmp, link))
}

func writeSystemdUnit() error {
	unit := fmt.Sprintf(`[Unit]
Description=%s service
After=network.target

[Service]
Type=simple
WorkingDirectory=%s/current
ExecStart=%s/current/%s
Restart=always
RestartSec=3
EnvironmentFile=%s/overwrite.env
Environment=DEPLOYD_SECRET=%s/secret.yaml
Environment=DEPLOYD_SERVICE=%s
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
`,
		serviceName,
		baseOpt,
		baseOpt,
		serviceName,
		baseEtc,
		baseEtc,
		serviceName,
	)

	if err := ensureDir(systemdPath); err != nil {
		return err
	}

	return os.WriteFile(systemdPath, []byte(unit), 0644)
}

func systemdReloadEnableRestart() error {
	ctx := context.Background()
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// if _, err = conn.ReloadContext(ctx); err != nil {
	// 	return err
	// }

	unit := serviceName + ".service"

	if _, _, err = conn.EnableUnitFilesContext(ctx, []string{unit}, false, true); err != nil {
		return err
	}

	ch := make(chan string, 1)
	_, err = conn.RestartUnitContext(ctx, unit, "replace", ch)
	if err != nil {
		return err
	}

	<-ch
	return nil
}

func touchFile(path string) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}
