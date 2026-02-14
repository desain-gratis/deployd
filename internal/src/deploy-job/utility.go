package deployjob

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// GPTMAXXING
func BuildUnit(namespace, service, description, executablePath string) string {
	pair := namespace + "_" + service

	return fmt.Sprintf(`[Unit]
Description=%s
After=network.target

[Service]
Type=simple
EnvironmentFile=-/etc/%s/env/overwrite.env
Environment=DEPLOYD_SERVICE_NAMESPACE=%v
Environment=DEPLOYD_SERVICE=%s
ExecStart=/opt/%s/current/%s
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
`, description, pair, namespace, service, pair, executablePath)
}

// ChatGPTMaxxing

func ExtractTarGzStrip(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip error: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar error: %w", err)
		}

		// Strip first path component
		cleanName := stripFirstLevel(hdr.Name)
		if cleanName == "" {
			continue
		}

		targetPath := filepath.Join(dest, cleanName)

		switch hdr.Typeflag {

		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(hdr.Mode)); err != nil {
				return err
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}

			out, err := os.OpenFile(
				targetPath,
				os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
				os.FileMode(hdr.Mode),
			)
			if err != nil {
				return err
			}

			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}

	return nil
}

func stripFirstLevel(path string) string {
	path = filepath.ToSlash(path)
	parts := strings.SplitN(path, "/", 2)

	if len(parts) < 2 {
		return ""
	}
	return parts[1]
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
