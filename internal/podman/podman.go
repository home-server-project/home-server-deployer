package podman

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/home-server-project/home-server-deployer/internal/model"
)

const (
	systemdEnv                    = "PODMAN_SYSTEMD_UNIT"
	SupportedMinimumVersion       = "5.8.2"
	quadletAPIIntroductionVersion = "5.8.0"
)

type Backend interface {
	DiscoverCapabilities(context.Context) (model.PodmanCapabilities, error)
	ListQuadlets(context.Context) ([]model.Quadlet, error)
	GetQuadlet(context.Context, string) (string, error)
	InstallQuadlet(context.Context, string, string) error
	RemoveQuadlet(context.Context, string, bool) error
	ListContainers(context.Context) ([]model.RuntimeContainer, error)
	Logs(context.Context, string, int) (string, error)
}

type HTTPClient struct {
	http          *http.Client
	base          string
	libpodAPI     string
	engineVersion string
}

func New(socket string) *HTTPClient {
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socket)
	}}
	return &HTTPClient{http: &http.Client{Transport: tr, Timeout: 30 * time.Second}, base: "http://podman"}
}

func NewWithHTTPClient(c *http.Client, base string) *HTTPClient {
	return &HTTPClient{http: c, base: strings.TrimRight(base, "/")}
}

type versionResponse struct {
	APIVersion    string `json:"ApiVersion"`
	MinAPIVersion string `json:"MinAPIVersion"`
	Version       string `json:"Version"`
	Arch          string `json:"Arch"`
	Components    []struct {
		Name    string            `json:"Name"`
		Version string            `json:"Version"`
		Details map[string]string `json:"Details"`
	} `json:"Components"`
}

func (c *HTTPClient) ensureVersion(ctx context.Context) (versionResponse, error) {
	var vr versionResponse
	if c.libpodAPI != "" {
		vr.Version = c.engineVersion
		vr.APIVersion = c.libpodAPI
		return vr, nil
	}
	resp, err := c.do(ctx, http.MethodGet, "/version", nil, nil, "")
	if err != nil {
		return vr, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return vr, httpError(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		return vr, fmt.Errorf("decode podman version: %w", err)
	}
	libpodAPI := ""
	for _, comp := range vr.Components {
		if comp.Name != "Podman Engine" {
			continue
		}
		libpodAPI = comp.Details["APIVersion"]
		if min := comp.Details["MinAPIVersion"]; min != "" {
			vr.MinAPIVersion = min
		}
		if vr.Version == "" {
			vr.Version = comp.Version
		}
		break
	}
	if libpodAPI == "" {
		return vr, errors.New("Podman /version did not report libpod APIVersion")
	}
	c.libpodAPI, c.engineVersion = libpodAPI, vr.Version
	return vr, nil
}

func (c *HTTPClient) libpodPath(p string) string { return "/v" + c.libpodAPI + p }

func (c *HTTPClient) DiscoverCapabilities(ctx context.Context) (model.PodmanCapabilities, error) {
	vr, err := c.ensureVersion(ctx)
	if err != nil {
		return model.PodmanCapabilities{}, err
	}
	caps := model.PodmanCapabilities{
		Version:             vr.Version,
		LibpodAPIVersion:    c.libpodAPI,
		MinLibpodAPIVersion: vr.MinAPIVersion,
		Architecture:        vr.Arch,
	}
	profile := ProfileForVersion(vr.Version)
	caps.NativeApplicationInstall = profile.NativeApplicationInstall

	resp, err := c.do(ctx, http.MethodGet, c.libpodPath("/libpod/quadlets/json"), nil, nil, "")
	if err == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			caps.QuadletList = true
			caps.QuadletPrint = profile.QuadletPrint
			caps.QuadletInstall = profile.QuadletInstall
			caps.QuadletRemove = profile.QuadletRemove
		}
	}

	infoResp, e := c.do(ctx, http.MethodGet, c.libpodPath("/libpod/info"), nil, nil, "")
	if e == nil {
		defer infoResp.Body.Close()
		if infoResp.StatusCode == http.StatusOK {
			var info struct {
				Host struct {
					Arch           string `json:"arch"`
					CgroupManager  string `json:"cgroupManager"`
					CgroupVersion  string `json:"cgroupVersion"`
					NetworkBackend string `json:"networkBackend"`
					Security       struct {
						Rootless bool `json:"rootless"`
					} `json:"security"`
				} `json:"host"`
			}
			if json.NewDecoder(infoResp.Body).Decode(&info) == nil {
				caps.Rootless = info.Host.Security.Rootless
				caps.CgroupManager = info.Host.CgroupManager
				caps.CgroupVersion = info.Host.CgroupVersion
				caps.NetworkBackend = info.Host.NetworkBackend
				if caps.Architecture == "" {
					caps.Architecture = info.Host.Arch
				}
			}
		}
	}
	return caps, nil
}

type CompatibilityProfile struct {
	QuadletPrint             bool
	QuadletInstall           bool
	QuadletRemove            bool
	NativeApplicationInstall bool
}

// ProfileForVersion describes API capability introduction points. It is not
// the Home Server Deployer product support policy.
func ProfileForVersion(v string) CompatibilityProfile {
	p := CompatibilityProfile{}
	if VersionAtLeast(v, quadletAPIIntroductionVersion) {
		p.QuadletPrint = true
		p.QuadletInstall = true
		p.QuadletRemove = true
	}
	if VersionAtLeast(v, "6.1.0") {
		p.NativeApplicationInstall = true
	}
	return p
}

// IsSupportedVersion applies Home Server Deployer's product support floor.
// This is intentionally separate from API capability discovery.
func IsSupportedVersion(v string) bool {
	return VersionAtLeast(v, SupportedMinimumVersion)
}

func VersionAtLeast(v, minimum string) bool {
	a, b, c, ok := parseSemver(v)
	if !ok {
		return false
	}
	x, y, z, ok := parseSemver(minimum)
	if !ok {
		return false
	}
	return greaterOrEqual(a, b, c, x, y, z)
}

func parseSemver(v string) (int, int, int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v = strings.SplitN(v, "-", 2)[0]
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return 0, 0, 0, false
	}
	vals := []int{0, 0, 0}
	for i := 0; i < len(parts) && i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return 0, 0, 0, false
		}
		vals[i] = n
	}
	return vals[0], vals[1], vals[2], true
}

func greaterOrEqual(a, b, c, x, y, z int) bool {
	if a != x {
		return a > x
	}
	if b != y {
		return b > y
	}
	return c >= z
}

func (c *HTTPClient) ListQuadlets(ctx context.Context) ([]model.Quadlet, error) {
	if _, err := c.ensureVersion(ctx); err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, http.MethodGet, c.libpodPath("/libpod/quadlets/json"), nil, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(resp)
	}
	var out []model.Quadlet
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *HTTPClient) GetQuadlet(ctx context.Context, name string) (string, error) {
	if !validName(name) {
		return "", errors.New("invalid quadlet name")
	}
	if _, err := c.ensureVersion(ctx); err != nil {
		return "", err
	}
	resp, err := c.do(ctx, http.MethodGet, c.libpodPath("/libpod/quadlets/"+url.PathEscape(name)+"/file"), nil, nil, "")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", httpError(resp)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return string(b), err
}

// InstallQuadlet always performs a normal install with systemd reload deferred.
// Alpha 0 deliberately does not use Podman's native replace path.
func (c *HTTPClient) InstallQuadlet(ctx context.Context, name, content string) error {
	if !validName(name) {
		return errors.New("invalid quadlet name")
	}
	if _, err := c.ensureVersion(ctx); err != nil {
		return err
	}
	var body bytes.Buffer
	tw := tar.NewWriter(&body)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
		return err
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	q := url.Values{"reload-systemd": {"false"}}
	resp, err := c.do(ctx, http.MethodPost, c.libpodPath("/libpod/quadlets"), q, bytes.NewReader(body.Bytes()), "application/x-tar")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return httpError(resp)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *HTTPClient) RemoveQuadlet(ctx context.Context, name string, force bool) error {
	if !validName(name) {
		return errors.New("invalid quadlet name")
	}
	if _, err := c.ensureVersion(ctx); err != nil {
		return err
	}
	q := url.Values{"reload-systemd": {"false"}, "ignore": {"false"}}
	if force {
		q.Set("force", "true")
	}
	resp, err := c.do(ctx, http.MethodDelete, c.libpodPath("/libpod/quadlets/"+url.PathEscape(name)), q, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return httpError(resp)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *HTTPClient) ListContainers(ctx context.Context) ([]model.RuntimeContainer, error) {
	if _, err := c.ensureVersion(ctx); err != nil {
		return nil, err
	}
	q := url.Values{"all": {"true"}}
	resp, err := c.do(ctx, http.MethodGet, c.libpodPath("/libpod/containers/json"), q, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(resp)
	}
	var list []struct {
		ID     string            `json:"Id"`
		Names  []string          `json:"Names"`
		State  string            `json:"State"`
		Status string            `json:"Status"`
		Labels map[string]string `json:"Labels"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	out := make([]model.RuntimeContainer, 0, len(list))
	for _, s := range list {
		name := ""
		if len(s.Names) > 0 {
			name = strings.TrimPrefix(s.Names[0], "/")
		}
		r := model.RuntimeContainer{ID: s.ID, Name: name, State: s.State, Status: s.Status, Labels: s.Labels, SystemdUnit: s.Labels[systemdEnv]}
		if insp, e := c.inspectContainer(ctx, s.ID); e == nil {
			if insp.Name != "" {
				r.Name = strings.TrimPrefix(insp.Name, "/")
			}
			if insp.State.Status != "" {
				r.State = insp.State.Status
			}
			r.Health = insp.State.Health.Status
			if r.SystemdUnit == "" {
				r.SystemdUnit = insp.Config.Labels[systemdEnv]
			}
			if r.SystemdUnit == "" {
				for _, env := range insp.Config.Env {
					if strings.HasPrefix(env, systemdEnv+"=") {
						r.SystemdUnit = strings.TrimPrefix(env, systemdEnv+"=")
						break
					}
				}
			}
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

type inspect struct {
	Name  string `json:"Name"`
	State struct {
		Status string `json:"Status"`
		Health struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
	Config struct {
		Env    []string          `json:"Env"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

func (c *HTTPClient) inspectContainer(ctx context.Context, id string) (inspect, error) {
	var v inspect
	resp, err := c.do(ctx, http.MethodGet, c.libpodPath("/libpod/containers/"+url.PathEscape(id)+"/json"), nil, nil, "")
	if err != nil {
		return v, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return v, httpError(resp)
	}
	err = json.NewDecoder(resp.Body).Decode(&v)
	return v, err
}

func (c *HTTPClient) Logs(ctx context.Context, id string, tail int) (string, error) {
	if tail <= 0 || tail > 5000 {
		tail = 200
	}
	if _, err := c.ensureVersion(ctx); err != nil {
		return "", err
	}
	q := url.Values{"stdout": {"true"}, "stderr": {"true"}, "tail": {strconv.Itoa(tail)}, "timestamps": {"false"}}
	resp, err := c.do(ctx, http.MethodGet, c.libpodPath("/libpod/containers/"+url.PathEscape(id)+"/logs"), q, nil, "")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", httpError(resp)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return string(b), err
}

func (c *HTTPClient) do(ctx context.Context, method, p string, q url.Values, body io.Reader, contentType string) (*http.Response, error) {
	u := c.base + p
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.http.Do(req)
}

func httpError(resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	return fmt.Errorf("podman API %s: %s", resp.Status, strings.TrimSpace(string(b)))
}

func validName(name string) bool {
	if name == "" || path.Base(name) != name || strings.ContainsAny(name, "/\\\r\n\x00") {
		return false
	}
	for _, e := range []string{".container", ".volume", ".network", ".image", ".build", ".pod", ".artifact"} {
		if strings.HasSuffix(name, e) {
			return true
		}
	}
	return false
}
