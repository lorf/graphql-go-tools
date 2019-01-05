package parser

import (
	"github.com/jensneuse/graphql-go-tools/pkg/document"
	"github.com/jensneuse/graphql-go-tools/pkg/lexing/keyword"
)

func (p *Parser) parseVariableDefinitions(index *[]int) (err error) {

	hasVariableDefinitions, err := p.peekExpect(keyword.BRACKETOPEN, true)
	if err != nil {
		return err
	}

	if !hasVariableDefinitions {
		return
	}

	for {
		next, err := p.l.Peek(true)
		if err != nil {
			return err
		}

		switch next {
		case keyword.VARIABLE:

			variable, err := p.l.Read()
			if err != nil {
				return err
			}

			variableDefinition := document.VariableDefinition{
				Variable: variable.Literal,
			}

			_, err = p.readExpect(keyword.COLON, "parseVariableDefinitions")
			if err != nil {
				return err
			}

			variableDefinition.Type, err = p.parseType()
			if err != nil {
				return err
			}

			variableDefinition.DefaultValue, err = p.parseDefaultValue()
			if err != nil {
				return err
			}

			*index = append(*index, p.putVariableDefinition(variableDefinition))

		case keyword.BRACKETCLOSE:
			_, err = p.l.Read()
			return err
		default:
			invalid, _ := p.l.Read()
			return newErrInvalidType(invalid.Position, "parseVariableDefinitions", "variable/bracket close", invalid.Keyword.String())
		}
	}
}