package systemd

import (
	"context"
	"fmt"
	"strings"
	"time"

	godbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/home-server-project/home-server-deployer/internal/model"
)

type Manager interface {
	Capabilities(context.Context) model.SystemdCapabilities
	Reload(context.Context) error
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Restart(context.Context, string) error
	Status(context.Context, string) (string, error)
	Close()
}

type Direct struct{ conn *godbus.Conn }

func NewDirect(ctx context.Context) (*Direct, error) { c,err:=godbus.NewSystemdConnectionContext(ctx); if err!=nil{return nil,err}; return &Direct{conn:c},nil }
func (d *Direct) Close(){if d!=nil&&d.conn!=nil{d.conn.Close()}}
func (d *Direct) Capabilities(ctx context.Context) model.SystemdCapabilities {_=ctx;c:=model.SystemdCapabilities{Available:d!=nil&&d.conn!=nil};if !c.Available{return c};if v,err:=d.conn.GetManagerProperty("Version");err==nil{c.Version=strings.Trim(v,"\"")};return c}
func (d *Direct) Reload(ctx context.Context) error{return d.conn.ReloadContext(ctx)}
func (d *Direct) Start(ctx context.Context,name string)error{return d.job(ctx,"start",name)}
func (d *Direct) Stop(ctx context.Context,name string)error{return d.job(ctx,"stop",name)}
func (d *Direct) Restart(ctx context.Context,name string)error{return d.job(ctx,"restart",name)}
func (d *Direct) job(ctx context.Context,op,name string)error{if !validUnit(name){return fmt.Errorf("invalid unit name %q",name)};ch:=make(chan string,1);var err error;switch op{case"start":_,err=d.conn.StartUnitContext(ctx,name,"replace",ch);case"stop":_,err=d.conn.StopUnitContext(ctx,name,"replace",ch);case"restart":_,err=d.conn.RestartUnitContext(ctx,name,"replace",ch);default:return fmt.Errorf("unknown operation")};if err!=nil{return err};select{case result:=<-ch:if result!="done"&&result!="skipped"{return fmt.Errorf("systemd %s %s: %s",op,name,result)};return nil;case<-ctx.Done():return ctx.Err();case<-time.After(30*time.Second):return fmt.Errorf("systemd %s %s timed out",op,name)}}
func (d *Direct) Status(ctx context.Context,name string)(string,error){if !validUnit(name){return "",fmt.Errorf("invalid unit name")};units,err:=d.conn.ListUnitsByNamesContext(ctx,[]string{name});if err!=nil{return "",err};if len(units)!=1{return "not-found",nil};u:=units[0];return fmt.Sprintf("%s/%s/%s",u.LoadState,u.ActiveState,u.SubState),nil}
func validUnit(name string)bool{return strings.HasSuffix(name,".service")&&!strings.ContainsAny(name,"/\\\r\n\x00")}
