package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	raven "github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
	"github.com/hugobrains/caasper/back"
	"github.com/hugobrains/caasper/front"
	"github.com/hugobrains/caasper/ir"
	"github.com/hugobrains/caasper/middle"
)

func main() {
	raven.SetDSN(os.Getenv("SENTRY_KEY"))

	router := gin.Default()

	// compiler frontend, handles lexing/parsing
	f := router.Group("/front")
	{
		// lexical analysis
		f.POST("/lex", front.Tokenize)

		// parsing.
		f.POST("/parse", front.Parse)
	}

	// intermediate representation, handles
	// conversion of AST into an IR
	i := router.Group("/ir")
	{
		i.POST("/build", ir.Build)
	}

	// 'middle' of compiler stages, takes the
	// krug IR and type checks everything.
	m := router.Group("/mid")
	{
		b := m.Group("/build")
		{
			// module -> [build_scope] -> scope map.
			b.POST("/scope", middle.BuildScope)

			// module -> [build_type] -> type map.
			b.POST("/type", middle.BuildType)
		}

		r := m.Group("/resolve")
		{
			// semantic module{module, scope_map, type_map} -> [type_resolve]
			//
			// this route takes in a module, scope map, and type map
			// and will resolve all of the types in top level declarations
			// e.g.
			// struct Person { name int, foo SomeStruct }
			// will check that all of the types in the struct resolve.
			r.POST("/type", middle.TypeResolve)

			// semantic module{module, scope_map, type_map} -> [symbol_resolve]
			//
			// this route takes in a moudle, scope map, and type map
			// it will check that all of the symbols exist in expressions
			r.POST("/symbol", middle.SymbolResolve)
		}
	}

	// backend of the compiler handles taking the
	// IR and generating code from it.
	b := router.Group("/back")
	{
		b.POST("/gen", back.Gen)
	}

	port := "8001"
	if p := os.Getenv("PORT"); p != "" {
		fmt.Println("Running on port", p)
		port = p
	}

	s := &http.Server{
		Addr:           fmt.Sprintf("localhost:%s", port),
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	s.ListenAndServe()
}
