go run client.go -s tethys.cs.umd.edu:8401 put status.html
go run client.go -s tethys.cs.umd.edu:8401 desc last
go run client.go -s tethys.cs.umd.edu:8401 -q key.private sign last
go run client.go -s tethys.cs.umd.edu:8401 desc last
go run client.go -s tethys.cs.umd.edu:8401 -p key.public verify last
go run client.go -s tethys.cs.umd.edu:8401 -p fake.public verify last
go run client.go -s tethys.cs.umd.edu:8401 -p key.public verify last