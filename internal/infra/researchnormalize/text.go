package researchnormalize

import (
	"context"
	"strings"
)

func normalizePlainText(ctx context.Context, builder *documentBuilder, document string) error {
	document = strings.ReplaceAll(document, "\r\n", "\n")
	document = strings.ReplaceAll(document, "\r", "\n")
	for _, block := range strings.Split(document, "\n\n") {
		if err := ctx.Err(); err != nil {
			return err
		}
		builder.addText(block)
		if builder.err != nil {
			return builder.err
		}
	}
	return nil
}
