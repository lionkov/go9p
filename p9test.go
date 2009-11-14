package main

import (
    "./p9";
	"./p9srv";
    "fmt"
)

func main() {
	fmt.Print(p9.IOHdrSz);	// 24
	fmt.Print(p9.Tremove);
	fmt.Print(p9.OAPPEND);	// 0x4000
	p9srv.ListenAndServe("tcp", "0.0.0.0:10000");
}
