echo p9
6g p9.go
echo p9srv
6g p9srv.go
echo p9cl
6g p9cl.go
echo p9test
6g p9test.go; 6l -o p9test p9test.6

echo p9ufs
6g p9ufs.go && 6l -o p9ufs p9ufs.6
