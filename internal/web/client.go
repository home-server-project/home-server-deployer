package web

import("bytes";"context";"encoding/json";"fmt";"io";"net";"net/http";"net/url";"time";"github.com/home-server-project/home-server-deployer/internal/model")
type AgentClient struct{http *http.Client}
func NewAgentClient(socket string)*AgentClient{tr:=&http.Transport{DialContext:func(ctx context.Context,_,_ string)(net.Conn,error){return (&net.Dialer{Timeout:3*time.Second}).DialContext(ctx,"unix",socket)}};return &AgentClient{http:&http.Client{Transport:tr,Timeout:20*time.Second}}}
func(c *AgentClient)get(ctx context.Context,p string,v any)error{return c.request(ctx,http.MethodGet,p,nil,v)}
func(c *AgentClient)post(ctx context.Context,p string,in,out any)error{b,err:=json.Marshal(in);if err!=nil{return err};return c.request(ctx,http.MethodPost,p,bytes.NewReader(b),out)}
func(c *AgentClient)request(ctx context.Context,method,p string,body io.Reader,out any)error{req,err:=http.NewRequestWithContext(ctx,method,"http://agent"+p,body);if err!=nil{return err};if body!=nil{req.Header.Set("Content-Type","application/json")};resp,err:=c.http.Do(req);if err!=nil{return err};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=300{b,_:=io.ReadAll(io.LimitReader(resp.Body,64<<10));return fmt.Errorf("agent %s: %s",resp.Status,bytes.TrimSpace(b))};if out==nil{io.Copy(io.Discard,resp.Body);return nil};return json.NewDecoder(resp.Body).Decode(out)}
func(c *AgentClient)Capabilities(ctx context.Context)(model.HostCapabilities,error){var v model.HostCapabilities;err:=c.get(ctx,"/v1/capabilities",&v);return v,err}
func(c *AgentClient)Catalog(ctx context.Context)([]model.Catalog,error){var v []model.Catalog;err:=c.get(ctx,"/v1/catalog",&v);return v,err}
func(c *AgentClient)Quadlets(ctx context.Context)([]model.DiscoveredQuadlet,error){var v []model.DiscoveredQuadlet;err:=c.get(ctx,"/v1/quadlets",&v);return v,err}
func(c *AgentClient)Instances(ctx context.Context)([]model.Instance,error){var v []model.Instance;err:=c.get(ctx,"/v1/instances",&v);return v,err}
func(c *AgentClient)Plan(ctx context.Context,r model.PlanRequest)(model.Plan,error){var v model.Plan;err:=c.post(ctx,"/v1/plans",r,&v);return v,err}
func(c *AgentClient)Execute(ctx context.Context,id string)(model.OperationResult,error){var v model.OperationResult;err:=c.post(ctx,"/v1/plans/"+url.PathEscape(id)+"/execute",map[string]string{},&v);return v,err}
func(c *AgentClient)Action(ctx context.Context,id,action string)error{return c.post(ctx,"/v1/instances/"+url.PathEscape(id)+"/actions/"+url.PathEscape(action),map[string]string{},&map[string]string{})}
func(c *AgentClient)Logs(ctx context.Context,id string)(map[string]string,error){var v map[string]string;err:=c.get(ctx,"/v1/instances/"+url.PathEscape(id)+"/logs",&v);return v,err}
func(c *AgentClient)Document(ctx context.Context,id,name string)(string,error){req,err:=http.NewRequestWithContext(ctx,http.MethodGet,"http://agent/v1/instances/"+url.PathEscape(id)+"/docs/"+url.PathEscape(name),nil);if err!=nil{return "",err};resp,err:=c.http.Do(req);if err!=nil{return "",err};defer resp.Body.Close();b,err:=io.ReadAll(io.LimitReader(resp.Body,2<<20));if err!=nil{return "",err};if resp.StatusCode<200||resp.StatusCode>=300{return "",fmt.Errorf("agent %s: %s",resp.Status,bytes.TrimSpace(b))};return string(b),nil}
