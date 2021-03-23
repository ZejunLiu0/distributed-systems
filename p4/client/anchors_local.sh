go run client.go -s localhost:8880 anchor zliu1238_root
go run client.go -s localhost:8880 put status.html 
go run client.go -s localhost:8880 claim zliu1238_root rootsig last
go run client.go -s localhost:8880 content zliu1238_root
go run client.go -s localhost:8880 get last x
cat x
go run client.go -s localhost:8880 put status2.html
go run client.go -s localhost:8880 claim zliu1238_root rootsig last
go run client.go -s localhost:8880 content zliu1238_root
go run client.go -s localhost:8880 get last x
cat x
go run client.go -s localhost:8880 rootanchor zliu1238_root
go run client.go -s localhost:8880 chain zliu1238_root
