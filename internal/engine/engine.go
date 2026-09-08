package engine

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/home-server-project/home-server-deployer/internal/catalog"
	"github.com/home-server-project/home-server-deployer/internal/config"
	"github.com/home-server-project/home-server-deployer/internal/docsgen"
	"github.com/home-server-project/home-server-deployer/internal/model"
	"github.com/home-server-project/home-server-deployer/internal/paths"
	"github.com/home-server-project/home-server-deployer/internal/podman"
	"github.com/home-server-project/home-server-deployer/internal/security"
	"github.com/home-server-project/home-server-deployer/internal/state"
	hostsystemd "github.com/home-server-project/home-server-deployer/internal/systemd"
)

type Engine struct {
	podman   podman.Backend
	systemd  hostsystemd.Manager
	catalog  *catalog.Store
	state    *state.Store
	docs     *docsgen.Generator
	security security.Backend
	roots    map[string]paths.Root
}

func New(p podman.Backend, sm hostsystemd.Manager, c *catalog.Store, st *state.Store, d *docsgen.Generator, sec security.Backend, cfg config.Config) *Engine {
	r := map[string]paths.Root{}
	for _, x := range cfg.ApprovedRoots { r[x.ID] = paths.Root{ID:x.ID,HostPath:x.HostPath,AgentPath:x.AgentPath} }
	return &Engine{p,sm,c,st,d,sec,r}
}
func (e *Engine) Capabilities(ctx context.Context)(model.HostCapabilities,error){p,err:=e.podman.DiscoverCapabilities(ctx);if err!=nil{return model.HostCapabilities{},err};h:=model.HostCapabilities{Podman:p,Systemd:e.systemd.Capabilities(ctx),Security:e.security.Capabilities()};ids:=make([]string,0,len(e.roots));for id:=range e.roots{ids=append(ids,id)};sort.Strings(ids);for _,id:=range ids{x:=e.roots[id];h.ApprovedRoots=append(h.ApprovedRoots,model.ApprovedRootInfo{ID:id,HostPath:x.HostPath})};return h,nil}
func(e *Engine)Catalog()[]model.Catalog{return e.catalog.List()}
func(e *Engine)Instances()([]model.Instance,error){return e.state.ListInstances()}
func(e *Engine)Discover(ctx context.Context)([]model.DiscoveredQuadlet,error){qs,err:=e.podman.ListQuadlets(ctx);if err!=nil{return nil,err};cs,err:=e.podman.ListContainers(ctx);if err!=nil{return nil,err};is,err:=e.state.ListInstances();if err!=nil{return nil,err};managed:=map[string]string{};for _,in:=range is{if in.RemovedAt==nil{for n:=range in.Resources{managed[n]=in.ID}}};byUnit:=map[string]model.RuntimeContainer{};for _,c:=range cs{if c.SystemdUnit!=""{byUnit[c.SystemdUnit]=c}};out:=make([]model.DiscoveredQuadlet,0,len(qs));for _,q:=range qs{d:=model.DiscoveredQuadlet{Quadlet:q};if id,ok:=managed[q.Name];ok{d.Managed=true;d.Instance=id};if c,ok:=byUnit[q.UnitName];ok{x:=c;d.Runtime=&x};out=append(out,d)};sort.Slice(out,func(i,j int)bool{return out[i].Quadlet.Name<out[j].Quadlet.Name});return out,nil}
func(e *Engine)CreatePlan(ctx context.Context,r model.PlanRequest)(model.Plan,error){if r.Operation!="install"&&r.Operation!="update"&&r.Operation!="remove"{return model.Plan{},fmt.Errorf("unsupported operation %q",r.Operation)};if !safeID(r.InstanceID){return model.Plan{},fmt.Errorf("invalid instanceId")};old,err:=e.state.GetInstance(r.InstanceID);exists:=err==nil;if err!=nil&&!os.IsNotExist(err){return model.Plan{},err};if r.Operation=="install"&&exists&&old.RemovedAt==nil{return model.Plan{},fmt.Errorf("instance %q already exists",r.InstanceID)};if r.Operation!="install"&&!exists{return model.Plan{},fmt.Errorf("instance %q is not managed",r.InstanceID)};appID:=r.AppID;if exists{if appID==""{appID=old.CatalogID};if appID!=old.CatalogID{return model.Plan{},fmt.Errorf("instance belongs to catalog %q",old.CatalogID)}};app,ok:=e.catalog.Get(appID);if !ok{return model.Plan{},fmt.Errorf("unknown catalog application %q",appID)};caps,err:=e.podman.DiscoverCapabilities(ctx);if err!=nil{return model.Plan{},err};if !caps.QuadletList||!caps.QuadletInstall||!caps.QuadletRemove{return model.Plan{},fmt.Errorf("required Quadlet API capabilities are unavailable")};if !versionAtLeast(caps.Version,app.Spec.MinPodmanVersion){return model.Plan{},fmt.Errorf("Podman %s is below application minimum %s",caps.Version,app.Spec.MinPodmanVersion)};params:=r.Parameters;if r.Operation!="install"&&len(params)==0{params=old.Parameters};params,err=catalog.ResolveParameters(app,params);if err!=nil{return model.Plan{},err};p:=model.Plan{Operation:r.Operation,AppID:appID,CatalogVersion:app.Metadata.Version,InstanceID:r.InstanceID,Parameters:params,CreatedAt:time.Now().UTC(),ExpiresAt:time.Now().UTC().Add(15*time.Minute)};p.ID,err=newID();if err!=nil{return p,err};if r.Operation=="remove"{p.Directories=old.Directories;for _,n:=range sortedResourceNames(old.Resources){x:=old.Resources[n];p.Resources=append(p.Resources,model.RenderedResource{Name:n,UnitName:x.UnitName,Content:x.Source,SHA256:x.SHA256,PreviousSHA256:x.SHA256})}}else{p.Directories,err=e.resolveDirs(app,params);if err!=nil{return p,err};p.Resources,err=e.render(app,p);if err!=nil{return p,err};if exists{for i:=range p.Resources{if x,ok:=old.Resources[p.Resources[i].Name];ok{p.Resources[i].PreviousSHA256=x.SHA256;p.Resources[i].UnitName=x.UnitName}}}};if exists&&old.RemovedAt==nil&&(r.Operation=="update"||r.Operation=="remove"){p.Drift=e.detectDrift(ctx,old)};p.Digest=planDigest(p);if err:=e.state.SavePlan(p);err!=nil{return p,err};return p,nil}
func(e *Engine)resolveDirs(app model.Catalog,p map[string]string)([]model.ResolvedDirectory,error){o:=[]model.ResolvedDirectory{};for _,d:=range app.Spec.Directories{r,ok:=e.roots[d.RootID];if !ok{return nil,fmt.Errorf("catalog requests unapproved root %q",d.RootID)};rel:=p[d.SubpathInput];agent,host,err:=paths.Resolve(r,rel);if err!=nil{return nil,err};s,err:=e.security.MountSuffix(d.SecurityIntent,d.ReadOnly);if err!=nil{return nil,err};o=append(o,model.ResolvedDirectory{Name:d.Name,RootID:d.RootID,RelativePath:rel,HostPath:host,AgentPath:agent,ContainerPath:d.ContainerPath,ReadOnly:d.ReadOnly,SecurityIntent:d.SecurityIntent,MountSuffix:s})};return o,nil}
type tplData struct{InstanceID string;Parameters map[string]string;Directories map[string]model.ResolvedDirectory}
func(e *Engine)render(app model.Catalog,p model.Plan)([]model.RenderedResource,error){dm:=map[string]model.ResolvedDirectory{};for _,d:=range p.Directories{dm[d.Name]=d};o:=[]model.RenderedResource{};for _,r:=range app.Spec.Resources{s,err:=e.catalog.Template(app.Metadata.ID,r.Template);if err!=nil{return nil,err};t,err:=template.New(r.Name).Option("missingkey=error").Parse(s);if err!=nil{return nil,err};var b strings.Builder;if err=t.Execute(&b,tplData{p.InstanceID,p.Parameters,dm});err!=nil{return nil,err};c:=fmt.Sprintf("# Managed-by: Home Server Deployer\n# Deployer-Instance: %s\n# Deployer-Catalog: %s@%s\n%s",p.InstanceID,app.Metadata.ID,app.Metadata.Version,b.String());o=append(o,model.RenderedResource{Name:r.Name,Content:c,Start:r.Start,SHA256:hash(c)})};return o,nil}
func(e *Engine)detectDrift(ctx context.Context,in model.Instance)[]model.DriftRecord{o:=[]model.DriftRecord{};for _,n:=range sortedResourceNames(in.Resources){want:=in.Resources[n].SHA256;got:="missing";if s,err:=e.podman.GetQuadlet(ctx,n);err==nil{got=hash(s)};if got!=want{o=append(o,model.DriftRecord{Resource:n,ExpectedHash:want,ActualHash:got})}};return o}
