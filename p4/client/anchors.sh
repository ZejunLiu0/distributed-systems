go run client.go -s tethys.cs.umd.edu:8400 anchor zliu1238_root
go run client.go -s tethys.cs.umd.edu:8400 put status.html 
go run client.go -s tethys.cs.umd.edu:8400 claim zliu1238_root rootsig last
go run client.go -s tethys.cs.umd.edu:8400 content zliu1238_root
go run client.go -s tethys.cs.umd.edu:8400 get last x
cat x
go run client.go -s tethys.cs.umd.edu:8400 put status2.html
go run client.go -s tethys.cs.umd.edu:8400 claim zliu1238_root rootsig last
go run client.go -s tethys.cs.umd.edu:8400 content zliu1238_root
go run client.go -s tethys.cs.umd.edu:8400 get last x
cat x
go run client.go -s tethys.cs.umd.edu:8400 rootanchor zliu1238_root
go run client.go -s tethys.cs.umd.edu:8400 chain zliu1238_root
