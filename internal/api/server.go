package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/home-server-project/home-server-deployer/internal/engine"
	"github.com/home-server-project/home-server-deployer/internal/model"
)

type Server struct{engine *engine.Engine;mux *http.ServeMux}
func New(e *engine.Engine)*Server{s:=&Server{engine:e,mux:http.NewServeMux()};s.routes();return s}
func(s *Server)Handler()http.Handler{return securityHeaders(s.mux)}
func(s *Server)routes(){s.mux.HandleFunc("GET /v1/health",func(w http.ResponseWriter,r *http.Request){writeJSON(w,200,map[string]string{"status":"ok"})});s.mux.HandleFunc("GET /v1/capabilities",func(w http.ResponseWriter,r *http.Request){v,err:=s.engine.Capabilities(r.Context());respond(w,v,err)});s.mux.HandleFunc("GET /v1/catalog",func(w http.ResponseWriter,r *http.Request){writeJSON(w,200,s.engine.Catalog())});s.mux.HandleFunc("GET /v1/quadlets",func(w http.ResponseWriter,r *http.Request){v,err:=s.engine.Discover(r.Context());respond(w,v,err)});s.mux.HandleFunc("GET /v1/instances",func(w http.ResponseWriter,r *http.Request){v,err:=s.engine.Instances();respond(w,v,err)});s.mux.HandleFunc("GET /v1/instances/{id}",func(w http.ResponseWriter,r *http.Request){v,err:=s.engine.Status(r.Context(),r.PathValue("id"));respond(w,v,err)});s.mux.HandleFunc("POST /v1/plans",s.plan);s.mux.HandleFunc("POST /v1/plans/{id}/execute",func(w http.ResponseWriter,r *http.Request){v,err:=s.engine.ExecutePlan(r.Context(),r.PathValue("id"));respond(w,v,err)});s.mux.HandleFunc("POST /v1/instances/{id}/actions/{action}",func(w http.ResponseWriter,r *http.Request){err:=s.engine.Lifecycle(r.Context(),r.PathValue("id"),r.PathValue("action"));respond(w,map[string]string{"status":"ok"},err)});s.mux.HandleFunc("GET /v1/instances/{id}/logs",s.logs);s.mux.HandleFunc("GET /v1/instances/{id}/docs/{name}",s.docs);s.mux.HandleFunc("GET /v1/instances/{id}/backup-contract",func(w http.ResponseWriter,r *http.Request){v,err:=s.engine.BackupContract(r.PathValue("id"));respond(w,v,err)});s.mux.HandleFunc("POST /v1/instances/{id}/backup",func(w http.ResponseWriter,r *http.Request){v,err:=s.engine.BackupContract(r.PathValue("id"));if err!=nil{writeError(w,statusFor(err),err);return};writeJSON(w,http.StatusNotImplemented,map[string]any{"status":"not-implemented","message":"backup execution is not implemented in Alpha 0","contract":v})})}
func(s *Server)plan(w http.ResponseWriter,r *http.Request){var req model.PlanRequest;if err:=decodeJSON(r,&req);err!=nil{writeError(w,400,err);return};v,err:=s.engine.CreatePlan(r.Context(),req);respond(w,v,err)}
func(s *Server)logs(w http.ResponseWriter,r *http.Request){tail:=200;if v:=r.URL.Query().Get("tail");v!=""{n,err:=strconv.Atoi(v);if err!=nil||n<1||n>5000{writeError(w,400,errors.New("tail must be 1..5000"));return};tail=n};v,err:=s.engine.Logs(r.Context(),r.PathValue("id"),tail);respond(w,v,err)}
func(s *Server)docs(w http.ResponseWriter,r *http.Request){name:=r.PathValue("name");b,err:=s.engine.Documentation(r.PathValue("id"),name);if err!=nil{writeError(w,statusFor(err),err);return};if name=="README.md"{w.Header().Set("Content-Type","text/markdown; charset=utf-8")}else{w.Header().Set("Content-Type","application/json")};w.WriteHeader(200);_,_=w.Write(b)}
func respond(w http.ResponseWriter,v any,err error){if err!=nil{writeError(w,statusFor(err),err);return};writeJSON(w,200,v)}
func statusFor(err error)int{s:=strings.ToLower(err.Error());switch{case strings.Contains(s,"not managed"),strings.Contains(s,"unknown catalog"),strings.Contains(s,"no such file"):return 404;case strings.Contains(s,"invalid"),strings.Contains(s,"unsupported"),strings.Contains(s,"required"),strings.Contains(s,"drift"),strings.Contains(s,"refusing"),strings.Contains(s,"already exists"),strings.Contains(s,"expired"):return 409;default:return 500}}
func decodeJSON(r *http.Request,v any)error{defer r.Body.Close();dec:=json.NewDecoder(io.LimitReader(r.Body,1<<20));dec.DisallowUnknownFields();if err:=dec.Decode(v);err!=nil{return fmt.Errorf("invalid JSON: %w",err)};if dec.Decode(&struct{}{})!=io.EOF{return errors.New("request must contain one JSON object")};return nil}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,status int,err error){writeJSON(w,status,map[string]string{"error":err.Error()})}
func securityHeaders(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("X-Content-Type-Options","nosniff");w.Header().Set("Cache-Control","no-store");next.ServeHTTP(w,r)})}
