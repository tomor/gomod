package reveal

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/Helcaraxan/gomod/lib/depgraph"
)

var (
	replaceA = Replacement{
		Offender: &depgraph.Module{Path: "offender"},
		Original: "originalA",
		Override: "overrideA",
		Version:  "v1.0.0",
	}
	replaceB = Replacement{
		Offender: moduleA,
		Original: "originalB",
		Override: "overrideB",
		Version:  "v1.0.0",
	}
	replaceC = Replacement{
		Offender: moduleA,
		Original: "originalC",
		Override: "./overrideC",
	}
	replaceD = Replacement{
		Offender: moduleA,
		Original: "originalD",
		Override: "./overrideD",
	}
	replaceE = Replacement{
		Offender: &depgraph.Module{Path: "offender-bis"},
		Original: "originalA",
		Override: "overrideA-bis",
		Version:  "v2.0.0",
	}
	replaceF = Replacement{
		Offender: &depgraph.Module{Path: "offender-tertio"},
		Original: "originalB",
		Override: "overrideB-bis",
		Version:  "v2.0.0",
	}

	testReplacements = &Replacements{
		main: "test-module",
		topLevel: map[string]string{
			"originalA": "overrideA",
			"originalB": "overrideB-bis",
		},
		replacedModules: []string{
			"originalA",
			"originalB",
			"originalC",
		},
		originToReplace: map[string][]Replacement{
			"originalA": {replaceA, replaceE},
			"originalB": {replaceB, replaceF},
			"originalC": {replaceC},
		},
	}

	moduleA = &depgraph.Module{
		Main:    false,
		Path:    "moduleA",
		Version: "v1.0.0",
		GoMod:   filepath.Join("testdata", "moduleA", "go.mod"),
	}
	moduleB = &depgraph.Module{
		Main:    false,
		Path:    filepath.Join("testdata", "moduleB"),
		Version: "v1.1.0",
	}
	moduleC = &depgraph.Module{
		Main:    false,
		Path:    "moduleA",
		Version: "v0.1.0",
		Replace: moduleA,
		GoMod:   "nowhere",
	}
	moduleD = &depgraph.Module{
		Main:    false,
		Path:    "moduleD",
		Version: "v0.0.1",
		GoMod:   "",
	}
)

var testGraph *depgraph.DepGraph

func init() {
	logger := logrus.New()
	logger.SetOutput(ioutil.Discard)

	testGraph = depgraph.NewGraph(logger, &depgraph.Module{
		Main:  true,
		Path:  "test/module",
		GoMod: filepath.Join("testdata", "mainModule", "go.mod"),
	})
	testGraph.AddNode(moduleA)
	testGraph.AddNode(moduleB)
	testGraph.AddNode(moduleC)
	testGraph.AddNode(moduleD)
}

func Test_ParseReplaces(t *testing.T) {
	logger := logrus.New()

	testcases := map[string]struct {
		input    string
		offender *depgraph.Module
		expected []Replacement
	}{
		"SingleReplace": {
			input:    "replace originalA => overrideA v1.0.0",
			offender: &depgraph.Module{Path: "offender"},
			expected: []Replacement{replaceA},
		},
		"MultiReplace": {
			input: `
replace (
	originalB => overrideB v1.0.0
	originalC => ./overrideC
)
`,
			offender: moduleA,
			expected: []Replacement{
				replaceB,
				replaceC,
			},
		},
		"MixedReplace": {
			input: `
replace (
	originalB => overrideB v1.0.0
	originalC => ./overrideC
)

replace originalD => ./overrideD
`,
			offender: moduleA,
			expected: []Replacement{
				replaceD,
				replaceB,
				replaceC,
			},
		},
		"FullGoMod": {
			input: `module github.com/foo/bar

go = 1.12.5

require (
	github.com/my-dep/A v1.2.0
	github.com/my-dep/B v1.9.2-201905291510-0123456789ab // indirect
	originalB v0.4.3
	originalC v0.2.3
	originalD v0.1.0
)

// Override this because it's upstream is broken.
replace originalC => ./overrideC // Bar

// Moar overrides.
replace (
	// Foo.
	originalB => overrideB v1.0.0
	originalD => ./overrideD
)
`,
			offender: moduleA,
			expected: []Replacement{
				replaceC,
				replaceB,
				replaceD,
			},
		},
	}

	for name, test := range testcases {
		t.Run(name, func(t *testing.T) {
			output := parseGoModForReplacements(logger, test.offender, test.input)
			assert.Equal(t, test.expected, output)
		})
	}
}

func Test_FindReplacements(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(ioutil.Discard)

	expectedReplacements := &Replacements{
		main:     "test/module",
		topLevel: map[string]string{"module/foo": "module/foo-bis"},
		replacedModules: []string{
			"originalB",
			"originalC",
			"originalD",
		},
		originToReplace: map[string][]Replacement{
			"originalB": {replaceB},
			"originalC": {replaceC},
			"originalD": {replaceD},
		},
	}

	replacements, err := FindReplacements(logger, testGraph)
	assert.NoError(t, err, "Should not error while searching for replacements.")
	assert.Equal(t, expectedReplacements, replacements, "Should find the expected replacement information.")
}

func Test_FilterReplacements(t *testing.T) {
	t.Run("OffenderEmpty", func(t *testing.T) {
		filtered := testReplacements.FilterOnOffendingModule(nil)
		assert.Equal(t, testReplacements, filtered, "Should return an identical array.")
	})
	t.Run("Offender", func(t *testing.T) {
		filtered := testReplacements.FilterOnOffendingModule([]string{"offender", "pre-offender", "offender-post"})
		assert.Equal(t, &Replacements{
			main: "test-module",
			topLevel: map[string]string{
				"originalA": "overrideA",
				"originalB": "overrideB-bis",
			},
			replacedModules: []string{
				"originalA",
			},
			originToReplace: map[string][]Replacement{
				"originalA": {replaceA},
			},
		}, filtered, "Should filter out the expected replacements.")
	})

	t.Run("OriginsEmpty", func(t *testing.T) {
		filtered := testReplacements.FilterOnReplacedModule(nil)
		assert.Equal(t, testReplacements, filtered, "Should return an identical array.")
	})
	t.Run("Origins", func(t *testing.T) {
		filtered := testReplacements.FilterOnReplacedModule([]string{"originalA", "originalC", "not-original"})
		assert.Equal(t, &Replacements{
			main: "test-module",
			topLevel: map[string]string{
				"originalA": "overrideA",
				"originalB": "overrideB-bis",
			},
			replacedModules: []string{
				"originalA",
				"originalC",
			},
			originToReplace: map[string][]Replacement{
				"originalA": {replaceA, replaceE},
				"originalC": {replaceC},
			},
		}, filtered, "Should filter out the expected replacements.")
	})
}

func Test_PrintReplacements(t *testing.T) {
	const expectedOutput = `'originalA' is replaced:
 ✓ offender     -> overrideA     @ v1.0.0
   offender-bis -> overrideA-bis @ v2.0.0

'originalB' is replaced:
   moduleA         -> overrideB     @ v1.0.0
 ✓ offender-tertio -> overrideB-bis @ v2.0.0

'originalC' is replaced:
   moduleA -> ./overrideC

[✓] Match with a top-level replace in 'test-module'
`

	logger := logrus.New()
	logger.SetOutput(ioutil.Discard)

	writer := &strings.Builder{}
	testReplacements.Print(logger, writer, nil, nil)
	assert.Equal(t, expectedOutput, writer.String(), "Should print the expected output.")
}

func Test_FindGoModFile(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(ioutil.Discard)

	testcases := map[string]struct {
		module         *depgraph.Module
		expectedModule *depgraph.Module
		expectedPath   string
	}{
		"NoModule": {
			module:         nil,
			expectedModule: nil,
			expectedPath:   "",
		},
		"Standard": {
			module:         moduleA,
			expectedModule: moduleA,
			expectedPath:   filepath.Join("testdata", "moduleA", "go.mod"),
		},
		"NoGoMod": {
			module:         moduleB,
			expectedModule: moduleB,
			expectedPath:   filepath.Join("testdata", "moduleB", "go.mod"),
		},
		"Replaced": {
			module:         moduleC,
			expectedModule: moduleA,
			expectedPath:   filepath.Join("testdata", "moduleA", "go.mod"),
		},
		"Invalid": {
			module:         moduleD,
			expectedModule: moduleD,
			expectedPath:   "",
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			module, goModPath := findGoModFile(logger, tc.module)
			assert.Equal(t, tc.expectedModule, module, "Should have determined the used module correctly.")
			assert.Equal(t, tc.expectedPath, goModPath, "Should have determined the correct go.mod path.")
		})
	}
}
