// Copyright 2024 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package tree

import (
	"sort"

	"github.com/cockroachdb/errors"
)

type CreateLogicalReplicationStream struct {
	PGURL   Expr
	From    LogicalReplicationResources
	Into    LogicalReplicationResources
	Options LogicalReplicationOptions
}

type LogicalReplicationResources struct {
	Tables   []*UnresolvedName
	Database Name
}

type LogicalReplicationOptions struct {
	// Mapping of table name to UDF name
	UserFunctions   map[TablePattern]Expr
	Cursor          Expr
	Mode            Expr
	DefaultFunction Expr
}

var _ Statement = &CreateLogicalReplicationStream{}
var _ NodeFormatter = &LogicalReplicationOptions{}

// Format implements the NodeFormatter interface.
func (node *CreateLogicalReplicationStream) Format(ctx *FmtCtx) {
	ctx.WriteString("CREATE LOGICAL REPLICATION STREAM FROM ")
	ctx.FormatNode(&node.From)
	ctx.WriteString(" ON ")
	ctx.FormatNode(node.PGURL)
	ctx.WriteString(" INTO ")
	ctx.FormatNode(&node.Into)

	if !node.Options.IsDefault() {
		ctx.WriteString(" WITH OPTIONS (")
		ctx.FormatNode(&node.Options)
		ctx.WriteString(")")
	}
}

func (lrr *LogicalReplicationResources) Format(ctx *FmtCtx) {
	if lrr.Database != "" {
		ctx.WriteString("DATABASE ")
		lrr.Database.Format(ctx)
	} else if len(lrr.Tables) > 1 {
		ctx.WriteString("TABLES (")
		for i := range lrr.Tables {
			if i > 0 {
				ctx.WriteString(", ")
			}
			ctx.FormatNode(lrr.Tables[i])
		}
		ctx.WriteString(")")
	} else {
		ctx.WriteString("TABLE ")
		ctx.FormatNode(lrr.Tables[0])
	}
}

func (lro *LogicalReplicationOptions) Format(ctx *FmtCtx) {
	var addSep bool
	maybeAddSep := func() {
		if addSep {
			ctx.WriteString(", ")
		}
		addSep = true
	}

	if lro.Cursor != nil {
		maybeAddSep()
		ctx.WriteString("CURSOR = ")
		ctx.FormatNode(lro.Cursor)
	}

	if lro.DefaultFunction != nil {
		maybeAddSep()
		ctx.WriteString("DEFAULT FUNCTION = ")
		ctx.FormatNode(lro.DefaultFunction)
	}

	if lro.Mode != nil {
		maybeAddSep()
		ctx.WriteString("MODE = ")
		ctx.FormatNode(lro.Mode)
	}

	if lro.UserFunctions != nil {
		maybeAddSep()
		addSep = false

		// In order to make tests deterministic, the ordering of map keys
		// needs to be the same each time.
		keys := make([]TablePattern, 0, len(lro.UserFunctions))
		for k := range lro.UserFunctions {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})

		for _, k := range keys {
			maybeAddSep()
			ctx.WriteString("FUNCTION ")
			ctx.FormatNode(lro.UserFunctions[k])
			ctx.WriteString(" FOR TABLE ")
			ctx.FormatNode(k)
		}
	}
}

func (o *LogicalReplicationOptions) CombineWith(other *LogicalReplicationOptions) error {
	if o.Cursor != nil {
		if other.Cursor != nil {
			return errors.New("CURSOR option specified multiple times")
		}
	} else {
		o.Cursor = other.Cursor
	}

	if o.Mode != nil {
		if other.Mode != nil {
			return errors.New("MODE option specified multiple times")
		}
	} else {
		o.Mode = other.Mode
	}

	if o.DefaultFunction != nil {
		if other.DefaultFunction != nil {
			return errors.New("DEFAULT FUNCTION option specified multiple times")
		}
	} else {
		o.DefaultFunction = other.DefaultFunction
	}

	if other.UserFunctions != nil {
		for tbl := range other.UserFunctions {
			if _, ok := o.UserFunctions[tbl]; ok {
				return errors.Newf("multiple user functions specified for table %q", tbl)
			}
			if o.UserFunctions == nil {
				o.UserFunctions = make(map[TablePattern]Expr)
			}
			o.UserFunctions[tbl] = other.UserFunctions[tbl]
		}
	}

	return nil
}

// IsDefault returns true if this logical options struct has default value.
func (o LogicalReplicationOptions) IsDefault() bool {
	options := LogicalReplicationOptions{}
	return o.Cursor == options.Cursor &&
		o.Mode == options.Mode &&
		o.DefaultFunction == options.DefaultFunction &&
		o.UserFunctions == nil
}
