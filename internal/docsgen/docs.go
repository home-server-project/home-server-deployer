package docsgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/home-server-project/home-server-deployer/internal/model"
)

type Generator struct{ root string }
func New(root string)*Generator{return &Generator{root:root}}
func (g *Generator) Write(instance model.Instance,app model.Catalog) error {dir:=filepath.Join(g.root,instance.ID);if err:=os.MkdirAll(dir,0750);err!=nil{return err};manifest,err:=json.MarshalIndent(instance,"","  ");if err!=nil{return err};if err:=atomic(filepath.Join(dir,"manifest.json"),append(manifest,'\n'));err!=nil{return err};var b strings.Builder;fmt.Fprintf(&b,"# %s\n\n",app.Metadata.Name);fmt.Fprintf(&b,"- Instance: `%s`\n- Catalog ID: `%s`\n- Catalog version: `%s`\n- Installed: `%s`\n",instance.ID,instance.CatalogID,instance.CatalogVersion,instance.InstalledAt.Format("2006-01-02 15:04:05Z07:00"));if instance.RemovedAt!=nil{fmt.Fprintf(&b,"- Removed: `%s`\n",instance.RemovedAt.Format("2006-01-02 15:04:05Z07:00"))};b.WriteString("\n## Quadlet resources\n\n");names:=make([]string,0,len(instance.Resources));for n:=range instance.Resources{names=append(names,n)};sort.Strings(names);for _,n:=range names{r:=instance.Resources[n];fmt.Fprintf(&b,"- `%s` -> `%s`, SHA-256 `%s`\n",r.Name,r.UnitName,r.SHA256);if r.Source!=""{b.WriteString("\n```ini\n");b.WriteString(r.Source);if !strings.HasSuffix(r.Source,"\n"){b.WriteByte('\n')};b.WriteString("```\n")}};b.WriteString("\n## Storage\n\n");for _,d:=range instance.Directories{fmt.Fprintf(&b,"- %s: `%s` -> `%s`, intent `%s`, mount options `%s`\n",d.Name,d.HostPath,d.ContainerPath,d.SecurityIntent,d.MountSuffix)};b.WriteString("\n## Update and health\n\n");fmt.Fprintf(&b,"- Update policy: `%s`",app.Spec.Update.Mode);if app.Spec.Update.Channel!=""{fmt.Fprintf(&b," (`%s`)",app.Spec.Update.Channel)};b.WriteByte('\n');fmt.Fprintf(&b,"- Health strategy: `%s`",app.Spec.Health.Strategy);if app.Spec.Health.PrimaryResource!=""{fmt.Fprintf(&b," (primary `%s`)",app.Spec.Health.PrimaryResource)};b.WriteByte('\n');b.WriteString("\n## Backup contract\n\n");fmt.Fprintf(&b,"- Strategy: `%s`\n- Consistency: `%s`\n- Provider: `%s`\n",app.Spec.Backup.Strategy,app.Spec.Backup.Consistency,app.Spec.Backup.Provider);if len(app.Spec.Backup.Includes)>0{fmt.Fprintf(&b,"- Includes: `%s`\n",strings.Join(app.Spec.Backup.Includes,"`, `"))};b.WriteString("\n> Native Quadlet files remain the source of truth. This document is recovery metadata generated from the last successfully observed installed state.\n");return atomic(filepath.Join(dir,"README.md"),[]byte(b.String()))}
func (g *Generator) Read(instanceID,name string)([]byte,error){if name!="README.md"&&name!="manifest.json"{return nil,fmt.Errorf("unsupported document")};return os.ReadFile(filepath.Join(g.root,instanceID,name))}
func atomic(p string,b []byte)error{dir:=filepath.Dir(p);f,err:=os.CreateTemp(dir,".doc-");if err!=nil{return err};n:=f.Name();defer os.Remove(n);if _,err:=f.Write(b);err!=nil{f.Close();return err};if err:=f.Sync();err!=nil{f.Close();return err};if err:=f.Close();err!=nil{return err};return os.Rename(n,p)}
