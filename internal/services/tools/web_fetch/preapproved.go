package web_fetch

import "strings"

// preapprovedHosts contains the set of domains that are auto-approved for WebFetch.
// These are code-related domains only; see the TypeScript source for the security rationale.
var preapprovedHosts = map[string]bool{
	"platform.claude.com":         true,
	"code.claude.com":             true,
	"modelcontextprotocol.io":     true,
	"agentskills.io":              true,
	"docs.python.org":             true,
	"en.cppreference.com":         true,
	"docs.oracle.com":             true,
	"learn.microsoft.com":         true,
	"developer.mozilla.org":       true,
	"go.dev":                      true,
	"pkg.go.dev":                  true,
	"www.php.net":                 true,
	"docs.swift.org":              true,
	"kotlinlang.org":              true,
	"ruby-doc.org":                true,
	"doc.rust-lang.org":           true,
	"www.typescriptlang.org":      true,
	"react.dev":                   true,
	"angular.io":                  true,
	"vuejs.org":                   true,
	"nextjs.org":                  true,
	"expressjs.com":               true,
	"nodejs.org":                  true,
	"bun.sh":                      true,
	"jquery.com":                  true,
	"getbootstrap.com":            true,
	"tailwindcss.com":             true,
	"d3js.org":                    true,
	"threejs.org":                 true,
	"redux.js.org":                true,
	"webpack.js.org":              true,
	"jestjs.io":                   true,
	"reactrouter.com":             true,
	"docs.djangoproject.com":      true,
	"flask.palletsprojects.com":   true,
	"fastapi.tiangolo.com":        true,
	"pandas.pydata.org":           true,
	"numpy.org":                   true,
	"www.tensorflow.org":          true,
	"pytorch.org":                 true,
	"scikit-learn.org":            true,
	"matplotlib.org":              true,
	"requests.readthedocs.io":     true,
	"jupyter.org":                 true,
	"laravel.com":                 true,
	"symfony.com":                 true,
	"wordpress.org":               true,
	"docs.spring.io":              true,
	"hibernate.org":               true,
	"tomcat.apache.org":           true,
	"gradle.org":                  true,
	"maven.apache.org":            true,
	"asp.net":                     true,
	"dotnet.microsoft.com":        true,
	"nuget.org":                   true,
	"blazor.net":                  true,
	"reactnative.dev":             true,
	"docs.flutter.dev":            true,
	"developer.apple.com":         true,
	"developer.android.com":       true,
	"keras.io":                    true,
	"spark.apache.org":            true,
	"huggingface.co":              true,
	"www.kaggle.com":              true,
	"www.mongodb.com":             true,
	"redis.io":                    true,
	"www.postgresql.org":          true,
	"dev.mysql.com":               true,
	"www.sqlite.org":              true,
	"graphql.org":                 true,
	"prisma.io":                   true,
	"docs.aws.amazon.com":         true,
	"cloud.google.com":            true,
	"kubernetes.io":               true,
	"www.docker.com":              true,
	"www.terraform.io":            true,
	"www.ansible.com":             true,
	"vercel.com":                  true,
	"docs.netlify.com":            true,
	"devcenter.heroku.com":        true,
	"cypress.io":                  true,
	"selenium.dev":                true,
	"docs.unity.com":              true,
	"docs.unrealengine.com":       true,
	"git-scm.com":                 true,
	"nginx.org":                   true,
	"httpd.apache.org":            true,
}

// preapprovedPathPrefixes maps hostnames to a list of path prefixes that are preapproved.
// For example, "github.com" is only preapproved for paths under "/anthropics".
var preapprovedPathPrefixes = map[string][]string{
	"github.com": {"/anthropics"},
	"vercel.com": {"/docs"},
}

// isPreapprovedHost reports whether the given hostname and pathname are in the preapproved list.
func isPreapprovedHost(hostname, pathname string) bool {
	if preapprovedHosts[hostname] {
		return true
	}
	prefixes, ok := preapprovedPathPrefixes[hostname]
	if !ok {
		return false
	}
	for _, p := range prefixes {
		if pathname == p || strings.HasPrefix(pathname, p+"/") {
			return true
		}
	}
	return false
}
