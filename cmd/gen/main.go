package main

import (
	"fmt"

	"github.com/pongsatt/go-rpc/tools"
)

func main() {
	generator := tools.NewGenerator()
	pkg := generator.LoadPackage()

	// 3. Proxy
	if generator.GenProxy {
		fmt.Println("generating proxy")

		infObjs := generator.ListInterfaces(pkg)

		for _, inf := range infObjs {
			infObj := generator.GetInfObj(pkg.Types.Name(), inf)
			f := generator.GenerateProxy(infObj)
			generator.WriteFile(f)
		}
	} else {
		fmt.Println("generating provider")

		// 4. Listener
		providerObjs := generator.ListStructs(pkg)
		for _, provider := range providerObjs {
			objInfo := generator.GetStructObj(pkg.Types.Name(), provider)
			f := generator.GenerateProvider(pkg.Types.Name(), objInfo)
			generator.WriteFile(f)
		}
	}

	fmt.Println("generate done")

}
