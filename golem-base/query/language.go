package query

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum/golem-base/arkivtype"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
)

type QueryOptions struct {
	AtBlock                     uint64
	IncludeAnnotations          bool
	IncludeSyntheticAnnotations bool
	Columns                     map[string]string
	OrderBy                     []arkivtype.OrderByAnnotation
	Cursor                      []arkivtype.CursorValue

	// Cache the sorted list of unique columns to fetch
	allColumnsSorted []string
	orderByColumns   []OrderBy
}

type OrderBy struct {
	Name       string
	Descending bool
}

func (opts *QueryOptions) GetColumnIndex(column string) (int, error) {
	ix, found := slices.BinarySearch(opts.AllColumns(), column)

	if !found {
		return -1, fmt.Errorf("unknown column %s", column)
	}
	return ix, nil
}

func (opts *QueryOptions) EncodeCursor(cursor *arkivtype.Cursor) (string, error) {
	encodedCursor := make([]any, 0, len(cursor.ColumnValues)*3+1)

	encodedCursor = append(encodedCursor, cursor.BlockNumber)

	for _, c := range cursor.ColumnValues {
		columnIx, err := opts.GetColumnIndex(c.ColumnName)
		if err != nil {
			return "", err
		}
		descending := uint64(0)
		if c.Descending {
			descending = 1
		}
		encodedCursor = append(encodedCursor,
			uint64(columnIx), c.Value, descending,
		)
	}

	s, err := json.Marshal(encodedCursor)
	if err != nil {
		return "", fmt.Errorf("could not marshal cursor: %w", err)
	}
	log.Info("Encoded cursor", "cursor", string(s))

	hexCursor := hex.EncodeToString([]byte(s))
	log.Info("Hex encoded cursor", "cursor", hexCursor)

	return hexCursor, nil
}

func (opts *QueryOptions) DecodeCursor(cursorStr string) (*arkivtype.Cursor, error) {
	if len(cursorStr) == 0 {
		return nil, nil
	}

	bs, err := hex.DecodeString(cursorStr)
	if err != nil {
		return nil, fmt.Errorf("could not decode cursor: %w", err)
	}

	cursor := arkivtype.Cursor{}

	encoded := make([]any, 0)
	err = json.Unmarshal(bs, &encoded)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal cursor: %w (%s)", err, string(bs))
	}

	firstValue, ok := encoded[0].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid block number: %d", encoded[0])
	}
	blockNumber := uint64(firstValue)
	cursor.BlockNumber = blockNumber

	cursor.ColumnValues = make([]arkivtype.CursorValue, 0, len(encoded)-1)

	for c := range slices.Chunk(encoded[1:], 3) {
		if len(c) != 3 {
			return nil, fmt.Errorf("invalid length of cursor array: %d", len(c))
		}

		firstValue, ok := c[0].(float64)
		if !ok {
			return nil, fmt.Errorf("unknown column index: %d", c[0])
		}
		thirdValue, ok := c[2].(float64)
		if !ok {
			return nil, fmt.Errorf("unknown value for descending: %d", c[3])
		}

		columnIx := int(firstValue)
		if columnIx >= len(opts.AllColumns()) {
			return nil, fmt.Errorf("unknown column index: %d", columnIx)
		}

		descendingInt := int(thirdValue)
		descending := false
		switch descendingInt {
		case 0:
			descending = false
		case 1:
			descending = true
		default:
			return nil, fmt.Errorf("unknown value for descending: %d", descendingInt)
		}

		cursor.ColumnValues = append(cursor.ColumnValues, arkivtype.CursorValue{
			ColumnName: opts.AllColumns()[columnIx],
			Value:      c[1],
			Descending: descending,
		})
	}

	jsonCursor, err := json.Marshal(cursor)
	if err != nil {
		return nil, err
	}
	log.Info("Decoded cursor", "cursor", string(jsonCursor))

	return &cursor, nil
}

func (opts *QueryOptions) AllColumns() []string {
	if opts.allColumnsSorted == nil {

		columns := slices.Collect(maps.Values(opts.Columns))

		for i := range opts.OrderBy {
			columns = append(columns, fmt.Sprintf("arkiv_annotation_sorting%d.value", i))
		}

		// We need the primary key of the entity table because of sorting
		columns = append(
			columns,
			arkivtype.GetColumnOrPanic("key"),
			arkivtype.GetColumnOrPanic("last_modified_at_block"),
			arkivtype.GetColumnOrPanic("transaction_index_in_block"),
			arkivtype.GetColumnOrPanic("operation_index_in_transaction"),
		)

		slices.Sort(columns)
		opts.allColumnsSorted = slices.Compact(columns)
	}

	return opts.allColumnsSorted
}

func (opts *QueryOptions) annotationSortingColumns() []OrderBy {
	columns := make([]OrderBy, 0, len(opts.OrderBy))
	for i, o := range opts.OrderBy {
		columns = append(columns, OrderBy{
			Name:       fmt.Sprintf("arkiv_annotation_sorting%d.value", i),
			Descending: o.Descending,
		})
	}
	return columns
}

func (opts *QueryOptions) OrderByColumns() []OrderBy {
	if opts.orderByColumns == nil {
		opts.orderByColumns = append(
			opts.annotationSortingColumns(),
			OrderBy{Name: arkivtype.GetColumnOrPanic("last_modified_at_block")},
			OrderBy{Name: arkivtype.GetColumnOrPanic("transaction_index_in_block")},
			OrderBy{Name: arkivtype.GetColumnOrPanic("operation_index_in_transaction")},
		)
	}
	return opts.orderByColumns
}

func (opts *QueryOptions) columnString() string {
	if len(opts.AllColumns()) == 0 {
		return "1"
	}
	return strings.Join(opts.AllColumns(), ", ")
}

// Define the lexer with distinct tokens for each operator and parentheses.
var lex = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Whitespace", Pattern: `[ \t\n\r]+`},
	{Name: "LParen", Pattern: `\(`},
	{Name: "RParen", Pattern: `\)`},
	{Name: "And", Pattern: `&&`},
	{Name: "Or", Pattern: `\|\|`},
	{Name: "Neq", Pattern: `!=`},
	{Name: "Eq", Pattern: `=`},
	{Name: "Geqt", Pattern: `>=`},
	{Name: "Leqt", Pattern: `<=`},
	{Name: "Gt", Pattern: `>`},
	{Name: "Lt", Pattern: `<`},
	{Name: "NotGlob", Pattern: `!~`},
	{Name: "Glob", Pattern: `~`},
	{Name: "Not", Pattern: `!`},
	{Name: "EntityKey", Pattern: `0x[a-fA-F0-9]{64}`},
	{Name: "Address", Pattern: `0x[a-fA-F0-9]{40}`},
	{Name: "String", Pattern: `"(?:[^"\\]|\\.)*"`},
	{Name: "Number", Pattern: `[0-9]+`},
	{Name: "Ident", Pattern: entity.AnnotationIdentRegex},
	// Meta-annotations, should start with $
	{Name: "Owner", Pattern: `\$owner`},
	{Name: "Creator", Pattern: `\$creator`},
	{Name: "Key", Pattern: `\$key`},
	{Name: "Expiration", Pattern: `\$expiration`},
	{Name: "Sequence", Pattern: `\$sequence`},
	{Name: "All", Pattern: `\$all`},
})

type SelectQuery struct {
	Query string
	Args  []any
}

type QueryBuilder struct {
	tableBuilder *strings.Builder
	args         []any
	needsWhere   bool
	options      QueryOptions
}

// WhereCondition represents a WHERE clause condition that can be combined with AND/OR
type WhereCondition struct {
	Condition string
}

func (b *QueryBuilder) addPaginationArguments() {
	args := []any{}
	paginationConditions := []string{}

	if len(b.options.Cursor) > 0 {
		for i := range b.options.Cursor {
			subcondition := []string{}
			for j, from := range b.options.Cursor {
				if j > i {
					break
				}
				var operator string
				if j < i {
					operator = "="
				} else if from.Descending {
					operator = "<"
				} else {
					operator = ">"
				}

				args = append(args, from.Value)

				subcondition = append(
					subcondition,
					fmt.Sprintf("%s %s ?", from.ColumnName, operator),
				)
			}

			paginationConditions = append(
				paginationConditions,
				fmt.Sprintf("(%s)", strings.Join(subcondition, " AND ")),
			)
		}

		paginationCondition := strings.Join(paginationConditions, " OR ")

		if b.needsWhere {
			b.tableBuilder.WriteString(" WHERE ")
			b.needsWhere = false
		} else {
			b.tableBuilder.WriteString(" AND ")
		}

		b.tableBuilder.WriteString(paginationCondition)
		b.args = append(b.args, args...)
	}
}

// createExistsCondition creates an EXISTS subquery condition for attribute filtering
func (b *QueryBuilder) createExistsCondition(
	attributeType string,
	whereClause string,
	arguments ...any,
) string {
	args := make([]any, 0, len(arguments)+2)
	// Add 2 AtBlock args: for a (FROM_BLOCK/TO_BLOCK)
	args = append(args, b.options.AtBlock, b.options.AtBlock)
	args = append(args, arguments...)

	tableName := "STRING_ATTRIBUTES"
	if attributeType == "numeric" {
		tableName = "NUMERIC_ATTRIBUTES"
	}

	// Build the WHERE clause with proper qualification
	clause := strings.ReplaceAll(strings.ReplaceAll(whereClause, "a.annotation_key", "a.KEY"), "annotation_key", "a.KEY")
	// Qualify unqualified KEY and VALUE with a.
	clause = strings.ReplaceAll(clause, " KEY ", " a.KEY ")
	clause = strings.ReplaceAll(clause, " VALUE ", " a.VALUE ")
	if strings.HasPrefix(clause, "KEY ") {
		clause = "a." + clause
	}
	if strings.HasPrefix(clause, "VALUE ") {
		clause = "a." + clause
	}

	existsQuery := fmt.Sprintf(
		"EXISTS (SELECT 1 FROM %s a WHERE a.ENTITY_KEY = e.ENTITY_KEY AND a.FROM_BLOCK = e.FROM_BLOCK AND a.FROM_BLOCK <= ? AND a.TO_BLOCK > ? AND %s)",
		tableName,
		clause,
	)

	b.args = append(b.args, args...)

	return existsQuery
}

type TopLevel struct {
	Expression *Expression `parser:"@@"`
	All        bool        `parser:"| @(All | '*')"`
}

func (t *TopLevel) Normalise() *TopLevel {
	if t.All {
		return t
	}
	return &TopLevel{
		Expression: t.Expression.Normalise(),
		All:        t.All,
	}
}

func (t *TopLevel) Evaluate(options *QueryOptions) (*SelectQuery, error) {
	tableBuilder := strings.Builder{}
	args := []any{}

	builder := QueryBuilder{
		options:      *options,
		tableBuilder: &tableBuilder,
		args:         args,
		needsWhere:   true,
	}

	// Build SELECT clause with proper column mappings from PAYLOADS
	columns := builder.options.AllColumns()
	selectParts := make([]string, 0, len(columns))
	for _, col := range columns {
		switch col {
		case "key":
			selectParts = append(selectParts, "lower(hex(e.ENTITY_KEY)) AS key")
		case "last_modified_at_block":
			selectParts = append(selectParts, "e.FROM_BLOCK AS last_modified_at_block")
		case "expires_at":
			selectParts = append(selectParts, "e.TO_BLOCK AS expires_at")
		case "created_at_block":
			selectParts = append(selectParts, "e.FROM_BLOCK AS created_at_block")
		case "payload":
			selectParts = append(selectParts, "e.PAYLOAD AS payload")
		case "transaction_index_in_block":
			selectParts = append(selectParts, "CAST(0 AS INTEGER) AS transaction_index_in_block")
		case "operation_index_in_transaction":
			selectParts = append(selectParts, "CAST(0 AS INTEGER) AS operation_index_in_transaction")
		case "content_type":
			// Default to application/octet-stream (content_type is not stored in temporal schema)
			selectParts = append(selectParts, "CAST('application/octet-stream' AS TEXT) AS content_type")
		case "owner_address":
			// Get from $owner attribute
			selectParts = append(selectParts, "(SELECT VALUE FROM STRING_ATTRIBUTES WHERE ENTITY_KEY = e.ENTITY_KEY AND FROM_BLOCK = e.FROM_BLOCK AND KEY = '$owner' AND FROM_BLOCK <= ? AND TO_BLOCK > ? LIMIT 1) AS owner_address")
		default:
			// Handle annotation sorting columns
			if strings.HasPrefix(col, "arkiv_annotation_sorting") && strings.HasSuffix(col, ".value") {
				// Use a valid alias without dots
				alias := strings.ReplaceAll(col, ".", "_")
				selectParts = append(selectParts, fmt.Sprintf("%s.VALUE AS %s", strings.TrimSuffix(col, ".value"), alias))
			} else {
				// Other columns (like from options.Columns)
				selectParts = append(selectParts, col)
			}
		}
	}

	columnSelect := strings.Join(selectParts, ", ")
	if columnSelect == "" {
		columnSelect = "1"
	}

	// Count how many AtBlock args we need for subqueries in SELECT (only for owner_address)
	// These need to be added first since they appear in the SELECT clause
	atBlockArgsCount := 0
	for _, col := range columns {
		if col == "owner_address" {
			atBlockArgsCount += 2 // Each subquery needs 2 AtBlock args
		}
	}
	// Add AtBlock args for subqueries in SELECT (before other args)
	for i := 0; i < atBlockArgsCount; i++ {
		builder.args = append(builder.args, builder.options.AtBlock, builder.options.AtBlock)
	}

	// Always start from PAYLOADS
	builder.tableBuilder.WriteString(strings.Join(
		[]string{
			"SELECT DISTINCT",
			columnSelect,
			"FROM",
			"PAYLOADS",
			"AS e",
		},
		" ",
	))

	// Add LEFT JOINs for sorting columns
	for i, orderBy := range builder.options.OrderBy {
		tableName := ""
		switch orderBy.Type {
		case "string":
			tableName = "STRING_ATTRIBUTES"
		case "numeric":
			tableName = "NUMERIC_ATTRIBUTES"
		default:
			return nil, fmt.Errorf("a type of either 'string' or 'numeric' needs to be provided for the annotation '%s'", orderBy.Name)
		}

		sortingTable := fmt.Sprintf("arkiv_annotation_sorting%d", i)
		fmt.Fprintf(builder.tableBuilder,
			" LEFT JOIN %s AS %s"+
				" ON %s.ENTITY_KEY = e.ENTITY_KEY"+
				" AND %s.FROM_BLOCK = e.FROM_BLOCK"+
				" AND %s.FROM_BLOCK <= ?"+
				" AND %s.TO_BLOCK > ?"+
				" AND %s.KEY = ?",

			tableName,
			sortingTable,
			sortingTable,
			sortingTable,
			sortingTable,
			sortingTable,
			sortingTable,
		)
		builder.args = append(builder.args, builder.options.AtBlock, builder.options.AtBlock, orderBy.Name)
	}

	// Build WHERE clause
	builder.tableBuilder.WriteString(" WHERE ")

	// Add temporal block conditions first
	builder.tableBuilder.WriteString("e.FROM_BLOCK <= ? AND e.TO_BLOCK > ?")
	builder.args = append(builder.args, builder.options.AtBlock, builder.options.AtBlock)

	// Add expression conditions if not querying all
	if !t.All {
		whereCond := t.Expression.Evaluate(&builder)
		if whereCond != nil && whereCond.Condition != "" {
			builder.tableBuilder.WriteString(" AND ")
			builder.tableBuilder.WriteString(whereCond.Condition)
		}
	}

	// Add pagination conditions
	builder.addPaginationArguments()

	builder.tableBuilder.WriteString(" ORDER BY ")

	orderColumns := make([]string, 0, len(builder.options.OrderByColumns()))
	for _, o := range builder.options.OrderByColumns() {
		suffix := ""
		if o.Descending {
			suffix = " DESC"
		}
		// Fix column names for ORDER BY - replace .value with _value for aliases
		orderCol := o.Name
		if strings.HasPrefix(orderCol, "arkiv_annotation_sorting") && strings.Contains(orderCol, ".value") {
			orderCol = strings.ReplaceAll(orderCol, ".", "_")
		}
		orderColumns = append(orderColumns, orderCol+suffix)
	}
	builder.tableBuilder.WriteString(strings.Join(orderColumns, ", "))

	return &SelectQuery{
		Query: builder.tableBuilder.String(),
		Args:  builder.args,
	}, nil
}

// Expression is the top-level rule.
type Expression struct {
	Or OrExpression `parser:"@@"`
}

func (e *Expression) Normalise() *Expression {
	normalised := e.Or.Normalise()
	// Remove unneeded OR+AND nodes that both only contain a single child
	// when that child is a parenthesised expression
	if len(normalised.Right) == 0 && len(normalised.Left.Right) == 0 && normalised.Left.Left.Paren != nil {
		// This has already been normalised by the call above, so any negation has
		// been pushed into the leaf expressions and we can safely strip away the
		// parentheses
		return &normalised.Left.Left.Paren.Nested
	}
	return &Expression{
		Or: *normalised,
	}
}

func (e *Expression) invert() *Expression {

	newLeft := e.Or.invert()

	if len(newLeft.Right) == 0 {
		// By construction, this will always be a Paren
		if newLeft.Left.Paren == nil {
			panic("This should never happen!")
		}
		return &newLeft.Left.Paren.Nested
	}

	return &Expression{
		Or: OrExpression{
			Left: *newLeft,
		},
	}
}

func (e *Expression) Evaluate(builder *QueryBuilder) *WhereCondition {
	return e.Or.Evaluate(builder)
}

// OrExpression handles expressions connected with ||.
type OrExpression struct {
	Left  AndExpression `parser:"@@"`
	Right []*OrRHS      `parser:"@@*"`
}

func (e *OrExpression) Normalise() *OrExpression {
	var newRight []*OrRHS = nil

	if e.Right != nil {
		newRight = make([]*OrRHS, 0, len(e.Right))
		for _, rhs := range e.Right {
			newRight = append(newRight, rhs.Normalise())
		}
	}

	return &OrExpression{
		Left:  *e.Left.Normalise(),
		Right: newRight,
	}
}

func (e *OrExpression) invert() *AndExpression {
	newLeft := EqualExpr{
		Paren: &Paren{
			IsNot: false,
			Nested: Expression{
				Or: *e.Left.invert(),
			},
		},
	}

	var newRight []*AndRHS = nil

	if e.Right != nil {
		newRight = make([]*AndRHS, 0, len(e.Right))
		for _, rhs := range e.Right {
			newRight = append(newRight, rhs.invert())
		}
	}

	return &AndExpression{
		Left:  newLeft,
		Right: newRight,
	}
}

func (e *OrExpression) Evaluate(b *QueryBuilder) *WhereCondition {
	leftCond := e.Left.Evaluate(b)
	conditions := []string{leftCond.Condition}

	for _, rhs := range e.Right {
		rightCond := rhs.Evaluate(b)
		conditions = append(conditions, rightCond.Condition)
	}

	// Combine OR conditions: (cond1) OR (cond2) OR ...
	if len(conditions) == 1 {
		return leftCond
	}

	combined := "(" + strings.Join(conditions, " OR ") + ")"
	return &WhereCondition{Condition: combined}
}

// OrRHS represents the right-hand side of an OR.
type OrRHS struct {
	Expr AndExpression `parser:"(Or | 'OR' | 'or') @@"`
}

func (e *OrRHS) Normalise() *OrRHS {
	return &OrRHS{
		Expr: *e.Expr.Normalise(),
	}
}

func (e *OrRHS) invert() *AndRHS {
	return &AndRHS{
		Expr: EqualExpr{
			Paren: &Paren{
				IsNot: false,
				Nested: Expression{
					Or: *e.Expr.invert(),
				},
			},
		},
	}
}

func (e *OrRHS) Evaluate(b *QueryBuilder) *WhereCondition {
	return e.Expr.Evaluate(b)
}

// AndExpression handles expressions connected with &&.
type AndExpression struct {
	Left  EqualExpr `parser:"@@"`
	Right []*AndRHS `parser:"@@*"`
}

func (e *AndExpression) Normalise() *AndExpression {
	var newRight []*AndRHS = nil

	if e.Right != nil {
		newRight = make([]*AndRHS, 0, len(e.Right))
		for _, rhs := range e.Right {
			newRight = append(newRight, rhs.Normalise())
		}
	}

	return &AndExpression{
		Left:  *e.Left.Normalise(),
		Right: newRight,
	}
}

func (e *AndExpression) invert() *OrExpression {
	newLeft := AndExpression{
		Left: *e.Left.invert(),
	}

	var newRight []*OrRHS = nil

	if e.Right != nil {
		newRight = make([]*OrRHS, 0, len(e.Right))
		for _, rhs := range e.Right {
			newRight = append(newRight, rhs.invert())
		}
	}

	return &OrExpression{
		Left:  newLeft,
		Right: newRight,
	}
}

func (e *AndExpression) Evaluate(b *QueryBuilder) *WhereCondition {
	leftCond := e.Left.Evaluate(b)
	conditions := []string{leftCond.Condition}

	for _, rhs := range e.Right {
		rightCond := rhs.Evaluate(b)
		conditions = append(conditions, rightCond.Condition)
	}

	// Combine AND conditions: (cond1) AND (cond2) AND ...
	if len(conditions) == 1 {
		return leftCond
	}

	combined := "(" + strings.Join(conditions, " AND ") + ")"
	return &WhereCondition{Condition: combined}
}

// AndRHS represents the right-hand side of an AND.
type AndRHS struct {
	Expr EqualExpr `parser:"(And | 'AND' | 'and') @@"`
}

func (e *AndRHS) Normalise() *AndRHS {
	return &AndRHS{
		Expr: *e.Expr.Normalise(),
	}
}

func (e *AndRHS) invert() *OrRHS {
	return &OrRHS{
		Expr: AndExpression{
			Left: *e.Expr.invert(),
		},
	}
}

func (e *AndRHS) Evaluate(b *QueryBuilder) *WhereCondition {
	return e.Expr.Evaluate(b)
}

// EqualExpr can be either an equality or a parenthesized expression.
type EqualExpr struct {
	Paren     *Paren     `parser:"  @@"`
	Assign    *Equality  `parser:"| @@"`
	Inclusion *Inclusion `parser:"| @@"`

	LessThan           *LessThan           `parser:"| @@"`
	LessOrEqualThan    *LessOrEqualThan    `parser:"| @@"`
	GreaterThan        *GreaterThan        `parser:"| @@"`
	GreaterOrEqualThan *GreaterOrEqualThan `parser:"| @@"`
	Glob               *Glob               `parser:"| @@"`
}

func (e *EqualExpr) Normalise() *EqualExpr {
	normalised := e

	if e.Paren != nil {
		p := e.Paren.Normalise()

		// Remove parentheses that only contain a single nested expression
		// (i.e. no OR or AND with multiple children)
		if len(p.Nested.Or.Right) == 0 && len(p.Nested.Or.Left.Right) == 0 {
			// This expression should already be properly normalised, we don't need to
			// call Normalise again here
			normalised = &p.Nested.Or.Left.Left
		} else {
			normalised = &EqualExpr{Paren: p}
		}
	}

	// Everything other than parenthesised expressions do not require further normalisation
	return normalised
}

func (e *EqualExpr) invert() *EqualExpr {
	if e.Paren != nil {
		return &EqualExpr{Paren: e.Paren.invert()}
	}

	if e.LessThan != nil {
		return &EqualExpr{GreaterOrEqualThan: e.LessThan.invert()}
	}

	if e.LessOrEqualThan != nil {
		return &EqualExpr{GreaterThan: e.LessOrEqualThan.invert()}
	}

	if e.GreaterThan != nil {
		return &EqualExpr{LessOrEqualThan: e.GreaterThan.invert()}
	}

	if e.GreaterOrEqualThan != nil {
		return &EqualExpr{LessThan: e.GreaterOrEqualThan.invert()}
	}

	if e.Glob != nil {
		return &EqualExpr{Glob: e.Glob.invert()}
	}

	if e.Assign != nil {
		return &EqualExpr{Assign: e.Assign.invert()}
	}

	if e.Inclusion != nil {
		return &EqualExpr{Inclusion: e.Inclusion.invert()}
	}

	panic("This should not happen!")
}

func (e *EqualExpr) Evaluate(b *QueryBuilder) *WhereCondition {
	if e.Paren != nil {
		return e.Paren.Evaluate(b)
	}

	if e.LessThan != nil {
		return e.LessThan.Evaluate(b)
	}

	if e.LessOrEqualThan != nil {
		return e.LessOrEqualThan.Evaluate(b)
	}

	if e.GreaterThan != nil {
		return e.GreaterThan.Evaluate(b)
	}

	if e.GreaterOrEqualThan != nil {
		return e.GreaterOrEqualThan.Evaluate(b)
	}

	if e.Glob != nil {
		return e.Glob.Evaluate(b)
	}

	if e.Assign != nil {
		return e.Assign.Evaluate(b)
	}

	if e.Inclusion != nil {
		return e.Inclusion.Evaluate(b)
	}

	panic("This should not happen!")
}

type Paren struct {
	IsNot  bool       `parser:"@(Not | 'NOT' | 'not')?"`
	Nested Expression `parser:"LParen @@ RParen"`
}

func (e *Paren) Normalise() *Paren {
	nested := e.Nested

	if e.IsNot {
		nested = *nested.invert()
	}

	return &Paren{
		IsNot:  false,
		Nested: *nested.Normalise(),
	}
}

func (e *Paren) invert() *Paren {
	return &Paren{
		IsNot:  !e.IsNot,
		Nested: e.Nested,
	}
}

func (e *Paren) Evaluate(b *QueryBuilder) *WhereCondition {
	expr := e.Nested
	// If we have a negation, we will push it down into the expression
	if e.IsNot {
		expr = *e.Nested.invert()
	}
	// We don't have to do anything here regarding precedence, the parsing order
	// is already taking care of precedence since the nested OR node will create a subquery
	return expr.Or.Evaluate(b)
}

type Glob struct {
	Var   string `parser:"@Ident"`
	IsNot bool   `parser:"((Glob | @NotGlob) | (@('NOT' | 'not')? ('GLOB' | 'glob')))"`
	Value string `parser:"@String"`
}

func (e *Glob) invert() *Glob {
	return &Glob{
		Var:   e.Var,
		IsNot: !e.IsNot,
		Value: e.Value,
	}
}

func (e *Glob) Evaluate(b *QueryBuilder) *WhereCondition {
	if !e.IsNot {
		condition := b.createExistsCondition(
			"string",
			strings.Join(
				[]string{
					"a.KEY = ?",
					"AND a.VALUE GLOB ?",
				},
				" ",
			),
			e.Var,
			e.Value,
		)
		return &WhereCondition{Condition: condition}
	} else {
		condition := b.createExistsCondition(
			"string",
			strings.Join(
				[]string{
					"a.KEY = ?",
					"AND a.VALUE NOT GLOB ?",
				},
				" ",
			),
			e.Var,
			e.Value,
		)
		return &WhereCondition{Condition: condition}
	}
}

type LessThan struct {
	Var   string `parser:"@Ident Lt"`
	Value Value  `parser:"@@"`
}

func (e *LessThan) invert() *GreaterOrEqualThan {
	return &GreaterOrEqualThan{
		Var:   e.Var,
		Value: e.Value,
	}
}

func (e *LessThan) Evaluate(b *QueryBuilder) *WhereCondition {
	if e.Value.String != nil {
		condition := b.createExistsCondition(
			"string",
			strings.Join(
				[]string{
					"a.KEY = ?",
					"AND a.VALUE < ?",
				},
				" ",
			),
			e.Var,
			*e.Value.String,
		)
		return &WhereCondition{Condition: condition}
	} else {
		condition := b.createExistsCondition(
			"numeric",
			strings.Join(
				[]string{
					"a.KEY = ?",
					"AND a.VALUE < ?",
				},
				" ",
			),
			e.Var,
			*e.Value.Number,
		)
		return &WhereCondition{Condition: condition}
	}
}

type LessOrEqualThan struct {
	Var   string `parser:"@Ident Leqt"`
	Value Value  `parser:"@@"`
}

func (e *LessOrEqualThan) invert() *GreaterThan {
	return &GreaterThan{
		Var:   e.Var,
		Value: e.Value,
	}
}

func (e *LessOrEqualThan) Evaluate(b *QueryBuilder) *WhereCondition {
	if e.Value.String != nil {
		condition := b.createExistsCondition(
			"string",
			strings.Join(
				[]string{
					"a.KEY = ?",
					"AND a.VALUE <= ?",
				},
				" ",
			),
			e.Var,
			*e.Value.String,
		)
		return &WhereCondition{Condition: condition}
	} else {
		condition := b.createExistsCondition(
			"numeric",
			strings.Join(
				[]string{
					"a.KEY = ?",
					"AND a.VALUE <= ?",
				},
				" ",
			),
			e.Var,
			*e.Value.Number,
		)
		return &WhereCondition{Condition: condition}
	}
}

type GreaterThan struct {
	Var   string `parser:"@Ident Gt"`
	Value Value  `parser:"@@"`
}

func (e *GreaterThan) invert() *LessOrEqualThan {
	return &LessOrEqualThan{
		Var:   e.Var,
		Value: e.Value,
	}
}

func (e *GreaterThan) Evaluate(b *QueryBuilder) *WhereCondition {
	if e.Value.String != nil {
		condition := b.createExistsCondition(
			"string",
			strings.Join(
				[]string{
					"a.KEY = ?",
					"AND a.VALUE > ?",
				},
				" ",
			),
			e.Var,
			*e.Value.String,
		)
		return &WhereCondition{Condition: condition}
	} else {
		condition := b.createExistsCondition(
			"numeric",
			strings.Join(
				[]string{
					"a.KEY = ?",
					"AND a.VALUE > ?",
				},
				" ",
			),
			e.Var,
			*e.Value.Number,
		)
		return &WhereCondition{Condition: condition}
	}
}

type GreaterOrEqualThan struct {
	Var   string `parser:"@Ident Geqt"`
	Value Value  `parser:"@@"`
}

func (e *GreaterOrEqualThan) invert() *LessThan {
	return &LessThan{
		Var:   e.Var,
		Value: e.Value,
	}
}

func (e *GreaterOrEqualThan) Evaluate(b *QueryBuilder) *WhereCondition {
	if e.Value.String != nil {
		condition := b.createExistsCondition(
			"string",
			strings.Join(
				[]string{
					"a.KEY = ?",
					"AND a.VALUE >= ?",
				},
				" ",
			),
			e.Var,
			*e.Value.String,
		)
		return &WhereCondition{Condition: condition}
	} else {
		condition := b.createExistsCondition(
			"numeric",
			strings.Join(
				[]string{
					"a.KEY = ?",
					"AND a.VALUE >= ?",
				},
				" ",
			),
			e.Var,
			*e.Value.Number,
		)
		return &WhereCondition{Condition: condition}
	}
}

// Equality represents a simple equality (e.g. name = 123).
type Equality struct {
	Var   string `parser:"@(Ident | Key | Owner | Creator | Expiration | Sequence)"`
	IsNot bool   `parser:"(Eq | @Neq)"`
	Value Value  `parser:"@@"`
}

func (e *Equality) invert() *Equality {
	return &Equality{
		Var:   e.Var,
		IsNot: !e.IsNot,
		Value: e.Value,
	}
}

func (e *Equality) Evaluate(b *QueryBuilder) *WhereCondition {
	if e.Value.String != nil {

		value := *e.Value.String
		if e.Var == arkivtype.OwnerAttributeKey ||
			e.Var == arkivtype.CreatorAttributeKey ||
			e.Var == arkivtype.KeyAttributeKey {
			value = strings.ToLower(value)
		}

		condition := "a.VALUE = ?"
		if e.IsNot {
			condition = "a.VALUE != ?"
		}

		existsCondition := b.createExistsCondition(
			"string",
			strings.Join(
				[]string{
					"a.KEY = ?",
					"AND",
					condition,
				},
				" ",
			),
			e.Var,
			value,
		)
		return &WhereCondition{Condition: existsCondition}

	} else {

		condition := "a.VALUE = ?"
		if e.IsNot {
			condition = "a.VALUE != ?"
		}

		existsCondition := b.createExistsCondition(
			"numeric",
			strings.Join(
				[]string{
					"a.KEY = ?",
					"AND",
					condition,
				},
				" ",
			),
			e.Var,
			*e.Value.Number,
		)
		return &WhereCondition{Condition: existsCondition}

	}
}

type Inclusion struct {
	Var    string `parser:"@(Ident | Key | Owner | Creator | Expiration | Sequence)"`
	IsNot  bool   `parser:"(@('NOT'|'not')? ('IN'|'in'))"`
	Values Values `parser:"@@"`
}

func (e *Inclusion) invert() *Inclusion {
	return &Inclusion{
		Var:    e.Var,
		IsNot:  !e.IsNot,
		Values: e.Values,
	}
}

func (e *Inclusion) Evaluate(b *QueryBuilder) *WhereCondition {
	if len(e.Values.Strings) > 0 {

		values := make([]any, 0, len(e.Values.Strings)+1)
		values = append(values, e.Var)
		for _, value := range e.Values.Strings {
			if e.Var == arkivtype.OwnerAttributeKey ||
				e.Var == arkivtype.CreatorAttributeKey ||
				e.Var == arkivtype.KeyAttributeKey {
				values = append(values, strings.ToLower(value))
			} else {
				values = append(values, value)
			}
		}

		paramStr := strings.Join(slices.Repeat([]string{"?"}, len(e.Values.Strings)), ", ")

		condition := fmt.Sprintf("a.VALUE IN (%s)", paramStr)
		if e.IsNot {
			condition = fmt.Sprintf("a.VALUE NOT IN (%s)", paramStr)
		}

		existsCondition := b.createExistsCondition(
			"string",
			strings.Join(
				[]string{
					"a.KEY = ?",
					"AND",
					condition,
				},
				" ",
			),
			values...,
		)
		return &WhereCondition{Condition: existsCondition}

	} else {

		values := make([]any, 0, len(e.Values.Numbers)+1)
		values = append(values, e.Var)
		for _, value := range e.Values.Numbers {
			values = append(values, value)
		}

		paramStr := strings.Join(slices.Repeat([]string{"?"}, len(e.Values.Numbers)), ", ")

		condition := fmt.Sprintf("a.VALUE IN (%s)", paramStr)
		if e.IsNot {
			condition = fmt.Sprintf("a.VALUE NOT IN (%s)", paramStr)
		}

		existsCondition := b.createExistsCondition(
			"numeric",
			strings.Join(
				[]string{
					"a.KEY = ?",
					"AND",
					condition,
				},
				" ",
			),
			values...,
		)
		return &WhereCondition{Condition: existsCondition}

	}
}

// Value is a literal value (a number or a string).
type Value struct {
	String *string `parser:"  (@String | @EntityKey | @Address)"`
	Number *uint64 `parser:"| @Number"`
}

type Values struct {
	Strings []string `parser:"  '(' (@String | @EntityKey | @Address)+ ')'"`
	Numbers []uint64 `parser:"| '(' @Number+ ')'"`
}

var Parser = participle.MustBuild[TopLevel](
	participle.Lexer(lex),
	participle.Elide("Whitespace"),
	participle.Unquote("String"),
)

func Parse(s string) (*TopLevel, error) {
	log.Info("Parsing query", "query", s)

	v, err := Parser.ParseString("", s)
	if err != nil {
		return nil, err
	}
	return v.Normalise(), err
}
