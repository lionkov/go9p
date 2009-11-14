package main

import (
    "./p9";
	"./p9srv";
    "fmt"
)

func main() {
	fmt.Println(p9.IOHdrSz);	// 24
	fmt.Println(p9.Tremove);
	fmt.Println(p9.OAPPEND);	// 0x4000
	p9srv.ListenAndServe("tcp", "0.0.0.0:10000");
}
