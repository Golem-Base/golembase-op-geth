package query

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/ethereum/go-ethereum/common"
)

// Define the lexer with distinct tokens for each operator and parentheses.
var lex = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Whitespace", Pattern: `[ \t\n\r]+`},
	{Name: "LParen", Pattern: `\(`},
	{Name: "RParen", Pattern: `\)`},
	{Name: "And", Pattern: `&&`},
	{Name: "Or", Pattern: `\|\|`},
	{Name: "Eq", Pattern: `=`},
	{Name: "String", Pattern: `"(?:[^"\\]|\\.)*"`},
	{Name: "Number", Pattern: `[0-9]+`},
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
})

// Expression is the top-level rule.
type Expression struct {
	Or *OrExpression `parser:"@@"`
}

func (e *Expression) Evaluate(ds DataSource) ([]common.Hash, error) {
	fmt.Println("Evaluate")
	return e.Or.Evaluate(ds)
}

// OrExpression handles expressions connected with ||.
type OrExpression struct {
	Left  *AndExpression `parser:"@@"`
	Right []*OrRHS       `parser:"@@*"`
}

func union(a, b []common.Hash) []common.Hash {
	result := make([]common.Hash, 0, len(a)+len(b))
	seen := make(map[common.Hash]bool)

	// Add all hashes from a
	for _, hash := range a {
		if !seen[hash] {
			seen[hash] = true
			result = append(result, hash)
		}
	}

	// Add any new hashes from b
	for _, hash := range b {
		if !seen[hash] {
			seen[hash] = true
			result = append(result, hash)
		}
	}

	return result
}

func (e *OrExpression) Evaluate(ds DataSource) ([]common.Hash, error) {
	fmt.Println("OR-Evaluate (left)")
	res, err := e.Left.Evaluate(ds)
	if err != nil {
		return nil, err
	}

	for _, rhs := range e.Right {
		fmt.Println("OR-Evaluate (right)")
		rh, err := rhs.Evaluate(ds)
		if err != nil {
			return nil, err
		}
		res = union(res, rh)
	}

	return res, nil
}

// OrRHS represents the right-hand side of an OR.
type OrRHS struct {
	Op   string         `parser:"@Or"`
	Expr *AndExpression `parser:"@@"`
}

func (e *OrRHS) Evaluate(ds DataSource) ([]common.Hash, error) {
	fmt.Println("OrRHS-Evaluate")
	return e.Expr.Evaluate(ds)
}

// AndExpression handles expressions connected with &&.
type AndExpression struct {
	Left  *EqualExpr `parser:"@@"`
	Right []*AndRHS  `parser:"@@*"`
}

func intersect(a, b []common.Hash) []common.Hash {
	result := make([]common.Hash, 0)
	seen := make(map[common.Hash]bool)

	// Build map of hashes in a
	for _, hash := range a {
		seen[hash] = true
	}

	// Check which hashes from b exist in map
	for _, hash := range b {
		if seen[hash] {
			result = append(result, hash)
		}
	}

	return result

}

func (e *AndExpression) Evaluate(ds DataSource) ([]common.Hash, error) {
	fmt.Println("AND-Evaluate (left)")
	res, err := e.Left.Evaluate(ds)
	if err != nil {
		return nil, err
	}

	for _, rhs := range e.Right {
		fmt.Println("AND-Evaluate (right)")
		rh, err := rhs.Evaluate(ds)
		if err != nil {
			return nil, err
		}
		res = intersect(res, rh)
	}

	return res, nil
}

// AndRHS represents the right-hand side of an AND.
type AndRHS struct {
	Op   string     `parser:"@And"`
	Expr *EqualExpr `parser:"@@"`
}

func (e *AndRHS) Evaluate(ds DataSource) ([]common.Hash, error) {
	fmt.Println("AndRHS-Evaluate")
	return e.Expr.Evaluate(ds)
}

// EqualExpr can be either an equality or a parenthesized expression.
type EqualExpr struct {
	Paren  *Expression `parser:"  \"(\" @@ \")\""`
	Assign *Equality   `parser:"| @@"`
}

func (e *EqualExpr) Evaluate(ds DataSource) ([]common.Hash, error) {
	fmt.Println("EQUAL-Evaluate")
	if e.Paren != nil {
		fmt.Println("EQUAL-Evaluate (paren)")
		return e.Paren.Evaluate(ds)
	}

	fmt.Println("EQUAL-Evaluate (Assign)")
	return e.Assign.Evaluate(ds)
}

// Equality represents a simple equality (e.g. name = 123).
type Equality struct {
	Var   string `parser:"@Ident \"=\""`
	Value *Value `parser:"@@"`
}

func (e *Equality) Evaluate(ds DataSource) ([]common.Hash, error) {
	fmt.Println("Equality-Evaluate")

	fmt.Println(e)

	if e.Value.String != nil {
		fmt.Printf("QUERY: Working with STRING %s %s\n", e.Var, *e.Value.String)

		if e.Var == "owner" && (strings.HasPrefix(*e.Value.String, "0x") || strings.HasPrefix(*e.Value.String, "0X")) {
			var addr common.Address

			decoded, err := hex.DecodeString((*e.Value.String)[2:])
			if err != nil {
				return nil, fmt.Errorf("invalid owner hex string: %w", err)
			}
			if len(decoded) != common.AddressLength {
				return nil, errors.New("invalid owner address length")
			}

			copy(addr[:], decoded)

			return ds.GetKeysForOwner(addr)
		}

		return ds.GetKeysForStringAnnotation(e.Var, *e.Value.String)
	}

	if e.Value.Number != nil {
		fmt.Printf("QUERY: Working with NUMBER %s %d\n", e.Var, *e.Value.Number)
		return ds.GetKeysForNumericAnnotation(e.Var, *e.Value.Number)
	}

	return nil, errors.New("unsupported value type")
}

// Value is a literal value (a number or a string).
type Value struct {
	String *string `parser:"  @String"`
	Number *uint64 `parser:"| @Number"`
}

var Parser = participle.MustBuild[Expression](
	participle.Lexer(lex),
	participle.Elide("Whitespace"),
	participle.Unquote("String"),
)

func Parse(s string) (*Expression, error) {
	v, err := Parser.ParseString("", s)
	return v, err
}
