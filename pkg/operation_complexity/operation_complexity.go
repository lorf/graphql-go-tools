/*
	package operation_complexity implements two common algorithms used by GitHub to calculate GraphQL query complexity

	1. Node count, the maximum number of Nodes a query may return
	2. Complexity, the maximum number of Node requests that might be needed to execute the query

	OperationComplexityEstimator takes a schema definition and a query and then walks recursively through the query to calculate both variables.

	The calculation can be influenced by integer arguments on fields that indicate the amount of Nodes returned by a field.

	To help the algorithm understand your schema you could make use of these two directives:

	- directive @nodeCountMultiply on ARGUMENT_DEFINITION
	- directive @nodeCountSkip on FIELD

	nodeCountMultiply:
	Indicates that the Int value the directive is applied on should be used as a Node multiplier

	nodeCountSkip:
	Indicates that the algorithm should skip this Node. This is useful to whitelist certain query paths, e.g. for introspection.
*/
package operation_complexity

import (
	"github.com/jensneuse/graphql-go-tools/pkg/ast"
	"github.com/jensneuse/graphql-go-tools/pkg/fastastvisitor"
)

var (
	nodeCountMultiply = []byte("nodeCountMultiply")
	nodeCountSkip     = []byte("nodeCountSkip")
)

type OperationComplexityEstimator struct {
	walker  *fastastvisitor.Walker
	visitor *complexityVisitor
}

func NewOperationComplexityEstimator() *OperationComplexityEstimator {

	walker := fastastvisitor.NewWalker(48)
	visitor := &complexityVisitor{
		Walker:      &walker,
		multipliers: make([]multiplier, 0, 16),
	}

	walker.RegisterEnterDocumentVisitor(visitor)
	walker.RegisterEnterArgumentVisitor(visitor)
	walker.RegisterLeaveFieldVisitor(visitor)
	walker.RegisterEnterFieldVisitor(visitor)
	walker.RegisterEnterSelectionSetVisitor(visitor)
	walker.RegisterEnterFragmentDefinitionVisitor(visitor)

	return &OperationComplexityEstimator{
		walker:  &walker,
		visitor: visitor,
	}
}

func (n *OperationComplexityEstimator) Do(operation, definition *ast.Document) (nodeCount, complexity int, err error) {
	n.visitor.count = 0
	n.visitor.complexity = 0
	n.visitor.multipliers = n.visitor.multipliers[:0]
	err = n.walker.Walk(operation, definition)
	return n.visitor.count, n.visitor.complexity, err
}

func CalculateOperationComplexity(operation, definition *ast.Document) (nodeCount, complexity int, err error) {
	estimator := NewOperationComplexityEstimator()
	return estimator.Do(operation, definition)
}

type complexityVisitor struct {
	*fastastvisitor.Walker
	operation, definition *ast.Document
	count                 int
	complexity            int
	multipliers           []multiplier
}

type multiplier struct {
	fieldRef int
	multi    int
}

func (c *complexityVisitor) calculateMultiplied(i int) int {
	for _, j := range c.multipliers {
		i = i * j.multi
	}
	return i
}

func (c *complexityVisitor) EnterDocument(operation, definition *ast.Document) {
	c.operation = operation
	c.definition = definition
}

func (c *complexityVisitor) EnterArgument(ref int) {

	if c.Ancestors[len(c.Ancestors)-1].Kind != ast.NodeKindField {
		return
	}

	definition, ok := c.ArgumentInputValueDefinition(ref)
	if !ok {
		return
	}

	if !c.definition.InputValueDefinitionHasDirective(definition, nodeCountMultiply) {
		return
	}

	value := c.operation.ArgumentValue(ref)
	if value.Kind == ast.ValueKindInteger {
		multi := c.operation.IntValueAsInt(value.Ref)
		c.multipliers = append(c.multipliers, multiplier{
			fieldRef: c.Ancestors[len(c.Ancestors)-1].Ref,
			multi:    multi,
		})
	}

	return
}

func (c *complexityVisitor) EnterField(ref int) {

	definition, exists := c.FieldDefinition(ref)

	if !exists {
		return
	}

	if _, exits := c.definition.FieldDefinitionDirectiveByName(definition, nodeCountSkip); exits {
		c.SkipNode()
		return
	}

	if !c.operation.FieldHasSelections(ref) {
		return
	}

	c.complexity = c.complexity + c.calculateMultiplied(1)

	return
}

func (c *complexityVisitor) LeaveField(ref int) {

	if len(c.multipliers) == 0 {
		return
	}

	if c.multipliers[len(c.multipliers)-1].fieldRef == ref {
		c.multipliers = c.multipliers[:len(c.multipliers)-1]
	}

	return
}

func (c *complexityVisitor) EnterSelectionSet(ref int) {

	if c.Ancestors[len(c.Ancestors)-1].Kind != ast.NodeKindField {
		return
	}

	c.count = c.count + c.calculateMultiplied(1)

	return
}

func (c *complexityVisitor) EnterFragmentDefinition(ref int) {
	c.SkipNode()
}
