package fangs

import (
	"reflect"

	"github.com/spf13/cobra"
)

func NewCommandFlagDescriptionProvider(tagName string, cmd *cobra.Command) DescriptionProvider {
	return &flagDescriptionProvider{
		tag:      tagName,
		flagRefs: collectFlagRefs(cmd),
	}
}

type flagDescriptionProvider struct {
	tag      string
	flagRefs flagRefs
}

var _ DescriptionProvider = (*flagDescriptionProvider)(nil)

func (d *flagDescriptionProvider) GetDescription(v reflect.Value, _ reflect.StructField) string {
	if v.CanAddr() {
		v = v.Addr()
		f := d.flagRefs[v.Pointer()]
		if f != nil {
			return f.Usage
		}
	}
	return ""
}

func collectFlagRefs(cmd *cobra.Command) flagRefs {
	out := getFlagRefs(cmd.PersistentFlags(), cmd.Flags())
	for _, c := range cmd.Commands() {
		for k, v := range collectFlagRefs(c) {
			out[k] = v
		}
	}
	return out
}
