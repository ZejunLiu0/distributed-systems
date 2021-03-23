go run client.go -s tethys.cs.umd.edu:8401 -q key.private anchor zliu1238
go run client.go -s tethys.cs.umd.edu:8401 desc last
go run client.go -s tethys.cs.umd.edu:8401 put status.html
go run client.go -s tethys.cs.umd.edu:8401 -q fake.private claim zliu1238 rootsig last
go run client.go -s tethys.cs.umd.edu:8401 put status.html
go run client.go -s tethys.cs.umd.edu:8401 -q key.private claim zliu1238 rootsig last
go run client.go -s tethys.cs.umd.edu:8401 desc last

go run client.go -s tethys.cs.umd.edu:8401 -q fake.private anchor zliu1238
go run client.go -s tethys.cs.umd.edu:8401 -q key.private anchor zliu1238



