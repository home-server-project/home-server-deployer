package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"html/template"
	"net/http"
	"strings"

	"github.com/home-server-project/home-server-deployer/internal/model"
)

type Server struct {
	agent *AgentClient
	mux   *http.ServeMux
	token string
	tmpl  *template.Template
}
type pageData struct {
	Token        string
	Error        string
	Message      string
	Capabilities model.HostCapabilities
	Catalog      []model.Catalog
	Instances    []model.Instance
	Quadlets     []model.DiscoveredQuadlet
	Plan         *model.Plan
	Logs         map[string]string
	Document     string
}

func NewServer(agent *AgentClient) (*Server, error) {
	tok, err := randomToken()
	if err != nil {
		return nil, err
	}
	t, err := template.New("page").Parse(pageTemplate)
	if err != nil {
		return nil, err
	}
	s := &Server{agent: agent, mux: http.NewServeMux(), token: tok, tmpl: t}
	s.routes()
	return s, nil
}
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
		s.mux.ServeHTTP(w, r)
	})
}
func (s *Server) routes() {
	s.mux.HandleFunc("GET /", s.index)
	s.mux.HandleFunc("POST /plan", s.withCSRF(s.plan))
	s.mux.HandleFunc("POST /execute", s.withCSRF(s.execute))
	s.mux.HandleFunc("POST /action", s.withCSRF(s.action))
	s.mux.HandleFunc("GET /logs/{id}", s.logs)
	s.mux.HandleFunc("GET /docs/{id}", s.docs)
}
func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	d := pageData{Token: s.token, Message: r.URL.Query().Get("message")}
	var err error
	if d.Capabilities, err = s.agent.Capabilities(r.Context()); err != nil {
		d.Error = err.Error()
		s.render(w, d)
		return
	}
	d.Catalog, _ = s.agent.Catalog(r.Context())
	d.Instances, _ = s.agent.Instances(r.Context())
	d.Quadlets, _ = s.agent.Quadlets(r.Context())
	s.render(w, d)
}
func (s *Server) plan(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.render(w, pageData{Token: s.token, Error: err.Error()})
		return
	}
	params := map[string]string{}
	for k, vs := range r.PostForm {
		if strings.HasPrefix(k, "param.") && len(vs) > 0 {
			params[strings.TrimPrefix(k, "param.")] = vs[0]
		}
	}
	p, err := s.agent.Plan(r.Context(), model.PlanRequest{Operation: r.FormValue("operation"), AppID: r.FormValue("appId"), InstanceID: r.FormValue("instanceId"), Parameters: params})
	if err != nil {
		s.render(w, pageData{Token: s.token, Error: err.Error()})
		return
	}
	s.render(w, pageData{Token: s.token, Plan: &p})
}
func (s *Server) execute(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	res, err := s.agent.Execute(r.Context(), r.FormValue("planId"))
	if err != nil {
		s.render(w, pageData{Token: s.token, Error: err.Error()})
		return
	}
	http.Redirect(w, r, "/?message="+res.Operation+"+succeeded", http.StatusSeeOther)
}
func (s *Server) action(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	err := s.agent.Action(r.Context(), r.FormValue("instanceId"), r.FormValue("action"))
	if err != nil {
		s.render(w, pageData{Token: s.token, Error: err.Error()})
		return
	}
	http.Redirect(w, r, "/?message=action+succeeded", http.StatusSeeOther)
}
func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.agent.Logs(r.Context(), r.PathValue("id"))
	d := pageData{Token: s.token, Logs: logs}
	if err != nil {
		d.Error = err.Error()
	}
	s.render(w, d)
}
func (s *Server) docs(w http.ResponseWriter, r *http.Request) {
	doc, err := s.agent.Document(r.Context(), r.PathValue("id"), "README.md")
	d := pageData{Token: s.token, Document: doc}
	if err != nil {
		d.Error = err.Error()
	}
	s.render(w, d)
}

func (s *Server) withCSRF(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || subtle.ConstantTimeCompare([]byte(r.FormValue("csrf")), []byte(s.token)) != 1 {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}
func (s *Server) render(w http.ResponseWriter, d pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(w, d); err != nil {
		http.Error(w, err.Error(), 500)
	}
}
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

const pageTemplate = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Home Server Deployer Alpha 0</title><style>body{font-family:system-ui,sans-serif;max-width:1100px;margin:2rem auto;padding:0 1rem;background:#111;color:#eee}section{border:1px solid #444;border-radius:10px;padding:1rem;margin:1rem 0}input,button,select{padding:.45rem;margin:.2rem;background:#222;color:#eee;border:1px solid #666;border-radius:5px}code,pre{background:#1b1b1b;padding:.2rem .35rem;border-radius:4px}pre{overflow:auto;white-space:pre-wrap}.error{color:#ff8d8d}.ok{color:#8dffac}.muted{color:#aaa}</style></head><body><h1>Home Server Deployer <small>Alpha 0</small></h1><p class="muted">Primitive engine/privilege-boundary test UI. Native Quadlets remain authoritative.</p>{{if .Error}}<p class="error">{{.Error}}</p>{{end}}{{if .Message}}<p class="ok">{{.Message}}</p>{{end}}{{if .Plan}}<section><h2>Execution plan</h2><p><b>{{.Plan.Operation}}</b> {{.Plan.AppID}} as <code>{{.Plan.InstanceID}}</code></p><p>Digest <code>{{.Plan.Digest}}</code></p>{{if .Plan.Drift}}<p class="error">Administrator drift detected. Execution will be refused.</p>{{range .Plan.Drift}}<pre>{{.Resource}} expected {{.ExpectedHash}} actual {{.ActualHash}}</pre>{{end}}{{end}}<h3>Directories</h3>{{range .Plan.Directories}}<p><code>{{.HostPath}}</code> → <code>{{.ContainerPath}}</code> ({{.SecurityIntent}}, {{.MountSuffix}})</p>{{end}}<h3>Quadlets</h3>{{range .Plan.Resources}}<details><summary>{{.Name}} — {{.SHA256}}{{if .PreviousSHA256}} (previous {{.PreviousSHA256}}){{end}}</summary><pre>{{.Content}}</pre></details>{{end}}<form method="post" action="/execute"><input type="hidden" name="csrf" value="{{$.Token}}"><input type="hidden" name="planId" value="{{.Plan.ID}}"><button type="submit">Execute approved plan</button></form><p><a href="/">Cancel</a></p></section>{{else if .Logs}}<section><h2>Logs</h2>{{range $name,$log:=.Logs}}<h3>{{$name}}</h3><pre>{{$log}}</pre>{{end}}<p><a href="/">Back</a></p></section>{{else if .Document}}<section><h2>Generated documentation</h2><pre>{{.Document}}</pre><p><a href="/">Back</a></p></section>{{else}}<section><h2>Host capabilities</h2><p>Podman <b>{{.Capabilities.Podman.Version}}</b> / Libpod API {{.Capabilities.Podman.LibpodAPIVersion}} · systemd {{.Capabilities.Systemd.Version}} · {{.Capabilities.Security.Backend}} enforcing={{.Capabilities.Security.Enforcing}}</p><p>Quadlet list/install/remove: {{.Capabilities.Podman.QuadletList}}/{{.Capabilities.Podman.QuadletInstall}}/{{.Capabilities.Podman.QuadletRemove}} · native application install={{.Capabilities.Podman.NativeApplicationInstall}}</p></section><section><h2>Catalog</h2>{{range .Catalog}}<article><h3>{{.Metadata.Name}}</h3><p>{{.Metadata.Description}}</p><form method="post" action="/plan"><input type="hidden" name="csrf" value="{{$.Token}}"><input type="hidden" name="operation" value="install"><input type="hidden" name="appId" value="{{.Metadata.ID}}"><label>Instance <input name="instanceId" value="{{.Metadata.ID}}" required></label>{{range .Spec.Inputs}}<label>{{.Name}} <input name="param.{{.Name}}" value="{{.Default}}" {{if .Required}}required{{end}}></label>{{end}}<button>Plan install</button></form></article>{{end}}</section><section><h2>Managed instances</h2>{{range .Instances}}<article><h3>{{.ID}}</h3><p>{{.CatalogID}} @ {{.CatalogVersion}}{{if .RemovedAt}} · removed{{end}}</p>{{if not .RemovedAt}}<form method="post" action="/plan" style="display:inline"><input type="hidden" name="csrf" value="{{$.Token}}"><input type="hidden" name="operation" value="update"><input type="hidden" name="appId" value="{{.CatalogID}}"><input type="hidden" name="instanceId" value="{{.ID}}"><button>Plan update</button></form><form method="post" action="/plan" style="display:inline"><input type="hidden" name="csrf" value="{{$.Token}}"><input type="hidden" name="operation" value="remove"><input type="hidden" name="appId" value="{{.CatalogID}}"><input type="hidden" name="instanceId" value="{{.ID}}"><button>Plan remove</button></form><form method="post" action="/action" style="display:inline"><input type="hidden" name="csrf" value="{{$.Token}}"><input type="hidden" name="instanceId" value="{{.ID}}"><input type="hidden" name="action" value="start"><button>start</button></form><form method="post" action="/action" style="display:inline"><input type="hidden" name="csrf" value="{{$.Token}}"><input type="hidden" name="instanceId" value="{{.ID}}"><input type="hidden" name="action" value="stop"><button>stop</button></form><form method="post" action="/action" style="display:inline"><input type="hidden" name="csrf" value="{{$.Token}}"><input type="hidden" name="instanceId" value="{{.ID}}"><input type="hidden" name="action" value="restart"><button>restart</button></form><a href="/logs/{{.ID}}">logs</a> · <a href="/docs/{{.ID}}">documentation</a>{{end}}</article>{{else}}<p>No managed instances.</p>{{end}}</section><section><h2>Discovered native Quadlets</h2>{{range .Quadlets}}<p><code>{{.Quadlet.Name}}</code> → {{.Quadlet.UnitName}} · {{.Quadlet.Status}} {{if .Managed}}· managed as {{.Instance}}{{else}}· external/read-only{{end}}{{if .Runtime}} · runtime {{.Runtime.Name}} {{.Runtime.State}}{{end}}</p>{{else}}<p>No Quadlets discovered.</p>{{end}}</section>{{end}}</body></html>`
