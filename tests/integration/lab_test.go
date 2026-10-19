//go:build integration

package integration

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	"github.com/AlexGalhardo/hacker-tools/internal/network"
)
func TestPrivateLab(t *testing.T){host:=os.Getenv("LAB_FIXTURE");if host==""{t.Fatal("LAB_FIXTURE is required; use docker compose lab")};ctx,cancel:=context.WithTimeout(t.Context(),20*time.Second);defer cancel();ips,e:=net.DefaultResolver.LookupIPAddr(ctx,host);if e!=nil||len(ips)==0{t.Fatal(ips,e)};address:=ips[0].IP.String();httpResult,e:=network.Diagnose(ctx,network.Request{Kind:"http",Target:"http://"+net.JoinHostPort(address,"8080")+"/health",TimeoutMS:1000});if e!=nil||httpResult.Values[0]!="204 No Content"{t.Fatal(httpResult,e)};inventory,e:=network.Discover(ctx,address+"/32",[]int{8080,9090,9091},500*time.Millisecond);if e!=nil||len(inventory)!=3{t.Fatal(inventory,e)};for _,r:=range inventory{want:="open";if r.Port==9091{want="unreachable"};if r.State!=want{t.Fatalf("port %d: %s",r.Port,r.State)}}
	original:=net.DefaultResolver;net.DefaultResolver=&net.Resolver{PreferGo:true,Dial:func(ctx context.Context,_,_ string)(net.Conn,error){return (&net.Dialer{}).DialContext(ctx,"udp",net.JoinHostPort(address,"5353"))}};defer func(){net.DefaultResolver=original}();result,e:=network.Diagnose(ctx,network.Request{Kind:"dns",Target:"fixture.test",TimeoutMS:1000});if e!=nil||strings.Join(result.Values,",")!="192.0.2.42"{t.Fatal(result,e)};_,e=network.Diagnose(ctx,network.Request{Kind:"dns",Target:"missing.test",TimeoutMS:1000});if dns,ok:=e.(*net.DNSError);!ok||!dns.IsNotFound{t.Fatal("expected NXDOMAIN",strconv.Quote(result.Target),e)}
}
