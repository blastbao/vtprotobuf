package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (p *vtproto) decodemessage(varName, buf string, desc protoreflect.Descriptor) {
	if strings.HasPrefix(string(desc.FullName()), "google.protobuf.") {
		p.P(`if err := `, p.Ident(ProtoPkg, "Unmarshal"), `(`, buf, `, `, varName, `); err != nil {`)
		p.P(`return err`)
		p.P(`}`)
		return
	}
	p.P(`if err := `, varName, `.UnmarshalVT(`, buf, `); err != nil {`)
	p.P(`return err`)
	p.P(`}`)
}

func (p *vtproto) decodeVarint(varName string, typName string) {
	p.P(`for shift := uint(0); ; shift += 7 {`)
	p.P(`if shift >= 64 {`)
	p.P(`return ErrIntOverflow` + p.localName)
	p.P(`}`)
	p.P(`if iNdEx >= l {`)
	p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
	p.P(`}`)
	p.P(`b := dAtA[iNdEx]`)
	p.P(`iNdEx++`)
	p.P(varName, ` |= `, typName, `(b&0x7F) << shift`)
	p.P(`if b < 0x80 {`)
	p.P(`break`)
	p.P(`}`)
	p.P(`}`)
}

func (p *vtproto) decodeFixed32(varName string, typeName string) {
	p.P(`if (iNdEx+4) > l {`)
	p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
	p.P(`}`)
	p.P(varName, ` = `, typeName, `(`, p.Ident("encoding/binary", "LittleEndian"), `.Uint32(dAtA[iNdEx:]))`)
	p.P(`iNdEx += 4`)
}

func (p *vtproto) decodeFixed64(varName string, typeName string) {
	p.P(`if (iNdEx+8) > l {`)
	p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
	p.P(`}`)
	p.P(varName, ` = `, typeName, `(`, p.Ident("encoding/binary", "LittleEndian"), `.Uint64(dAtA[iNdEx:]))`)
	p.P(`iNdEx += 8`)
}

func fieldGoType(g *protogen.GeneratedFile, field *protogen.Field) (goType string, pointer bool) {
	if field.Desc.IsWeak() {
		return "struct{}", false
	}

	pointer = field.Desc.HasPresence()
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		goType = "bool"
	case protoreflect.EnumKind:
		goType = g.QualifiedGoIdent(field.Enum.GoIdent)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		goType = "int32"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		goType = "uint32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		goType = "int64"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		goType = "uint64"
	case protoreflect.FloatKind:
		goType = "float32"
	case protoreflect.DoubleKind:
		goType = "float64"
	case protoreflect.StringKind:
		goType = "string"
	case protoreflect.BytesKind:
		goType = "[]byte"
		pointer = false // rely on nullability of slices for presence
	case protoreflect.MessageKind, protoreflect.GroupKind:
		goType = "*" + g.QualifiedGoIdent(field.Message.GoIdent)
		pointer = false // pointer captured as part of the type
	}
	switch {
	case field.Desc.IsList():
		return "[]" + goType, false
	case field.Desc.IsMap():
		keyType, _ := fieldGoType(g, field.Message.Fields[0])
		valType, _ := fieldGoType(g, field.Message.Fields[1])
		return fmt.Sprintf("map[%v]%v", keyType, valType), false
	}
	return goType, pointer
}

func (p *vtproto) declareMapField(varName string, nullable bool, field *protogen.Field) {
	switch field.Desc.Kind() {
	case protoreflect.DoubleKind:
		p.P(`var `, varName, ` float64`)
	case protoreflect.FloatKind:
		p.P(`var `, varName, ` float32`)
	case protoreflect.Int64Kind:
		p.P(`var `, varName, ` int64`)
	case protoreflect.Uint64Kind:
		p.P(`var `, varName, ` uint64`)
	case protoreflect.Int32Kind:
		p.P(`var `, varName, ` int32`)
	case protoreflect.Fixed64Kind:
		p.P(`var `, varName, ` uint64`)
	case protoreflect.Fixed32Kind:
		p.P(`var `, varName, ` uint32`)
	case protoreflect.BoolKind:
		p.P(`var `, varName, ` bool`)
	case protoreflect.StringKind:
		p.P(`var `, varName, ` `, field.GoIdent)
	case protoreflect.MessageKind:
		msgname := field.GoIdent
		if nullable {
			p.P(`var `, varName, ` *`, msgname)
		} else {
			p.P(varName, ` := &`, msgname, `{}`)
		}
	case protoreflect.BytesKind:
		p.P(varName, ` := []byte{}`)
	case protoreflect.Uint32Kind:
		p.P(`var `, varName, ` uint32`)
	case protoreflect.EnumKind:
		p.P(`var `, varName, ` `, field.GoIdent)
	case protoreflect.Sfixed32Kind:
		p.P(`var `, varName, ` int32`)
	case protoreflect.Sfixed64Kind:
		p.P(`var `, varName, ` int64`)
	case protoreflect.Sint32Kind:
		p.P(`var `, varName, ` int32`)
	case protoreflect.Sint64Kind:
		p.P(`var `, varName, ` int64`)
	}
}

func (p *vtproto) mapField(varName string, field *protogen.Field) {
	switch field.Desc.Kind() {
	case protoreflect.DoubleKind:
		p.P(`var `, varName, `temp uint64`)
		p.decodeFixed64(varName+"temp", "uint64")
		p.P(varName, ` = `, p.Ident("math", "Float64frombits"), `(`, varName, `temp)`)
	case protoreflect.FloatKind:
		p.P(`var `, varName, `temp uint32`)
		p.decodeFixed32(varName+"temp", "uint32")
		p.P(varName, ` = `, p.Ident("math", "Float32frombits"), `(`, varName, `temp)`)
	case protoreflect.Int64Kind:
		p.decodeVarint(varName, "int64")
	case protoreflect.Uint64Kind:
		p.decodeVarint(varName, "uint64")
	case protoreflect.Int32Kind:
		p.decodeVarint(varName, "int32")
	case protoreflect.Fixed64Kind:
		p.decodeFixed64(varName, "uint64")
	case protoreflect.Fixed32Kind:
		p.decodeFixed32(varName, "uint32")
	case protoreflect.BoolKind:
		p.P(`var `, varName, `temp int`)
		p.decodeVarint(varName+"temp", "int")
		p.P(varName, ` = bool(`, varName, `temp != 0)`)
	case protoreflect.StringKind:
		p.P(`var stringLen`, varName, ` uint64`)
		p.decodeVarint("stringLen"+varName, "uint64")
		p.P(`intStringLen`, varName, ` := int(stringLen`, varName, `)`)
		p.P(`if intStringLen`, varName, ` < 0 {`)
		p.P(`return ErrInvalidLength` + p.localName)
		p.P(`}`)
		p.P(`postStringIndex`, varName, ` := iNdEx + intStringLen`, varName)
		p.P(`if postStringIndex`, varName, ` < 0 {`)
		p.P(`return ErrInvalidLength` + p.localName)
		p.P(`}`)
		p.P(`if postStringIndex`, varName, ` > l {`)
		p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
		p.P(`}`)
		p.P(varName, ` = `, "string", `(dAtA[iNdEx:postStringIndex`, varName, `])`)
		p.P(`iNdEx = postStringIndex`, varName)
	case protoreflect.MessageKind:
		p.P(`var mapmsglen int`)
		p.decodeVarint("mapmsglen", "int")
		p.P(`if mapmsglen < 0 {`)
		p.P(`return ErrInvalidLength` + p.localName)
		p.P(`}`)
		p.P(`postmsgIndex := iNdEx + mapmsglen`)
		p.P(`if postmsgIndex < 0 {`)
		p.P(`return ErrInvalidLength` + p.localName)
		p.P(`}`)
		p.P(`if postmsgIndex > l {`)
		p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
		p.P(`}`)
		buf := `dAtA[iNdEx:postmsgIndex]`
		p.P(varName, ` = &`, p.noStarOrSliceType(field), `{}`)
		p.decodemessage(varName, buf, field.Message.Desc)
		p.P(`iNdEx = postmsgIndex`)
	case protoreflect.BytesKind:
		p.P(`var mapbyteLen uint64`)
		p.decodeVarint("mapbyteLen", "uint64")
		p.P(`intMapbyteLen := int(mapbyteLen)`)
		p.P(`if intMapbyteLen < 0 {`)
		p.P(`return ErrInvalidLength` + p.localName)
		p.P(`}`)
		p.P(`postbytesIndex := iNdEx + intMapbyteLen`)
		p.P(`if postbytesIndex < 0 {`)
		p.P(`return ErrInvalidLength` + p.localName)
		p.P(`}`)
		p.P(`if postbytesIndex > l {`)
		p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
		p.P(`}`)
		p.P(varName, ` = make([]byte, mapbyteLen)`)
		p.P(`copy(`, varName, `, dAtA[iNdEx:postbytesIndex])`)
		p.P(`iNdEx = postbytesIndex`)
	case protoreflect.Uint32Kind:
		p.decodeVarint(varName, "uint32")
	case protoreflect.EnumKind:
		goTypV, _ := fieldGoType(p.GeneratedFile, field)
		p.decodeVarint(varName, goTypV)
	case protoreflect.Sfixed32Kind:
		p.decodeFixed32(varName, "int32")
	case protoreflect.Sfixed64Kind:
		p.decodeFixed64(varName, "int64")
	case protoreflect.Sint32Kind:
		p.P(`var `, varName, `temp int32`)
		p.decodeVarint(varName+"temp", "int32")
		p.P(varName, `temp = int32((uint32(`, varName, `temp) >> 1) ^ uint32(((`, varName, `temp&1)<<31)>>31))`)
		p.P(varName, ` = int32(`, varName, `temp)`)
	case protoreflect.Sint64Kind:
		p.P(`var `, varName, `temp uint64`)
		p.decodeVarint(varName+"temp", "uint64")
		p.P(varName, `temp = (`, varName, `temp >> 1) ^ uint64((int64(`, varName, `temp&1)<<63)>>63)`)
		p.P(varName, ` = int64(`, varName, `temp)`)
	}
}

func (p *vtproto) noStarOrSliceType(field *protogen.Field) string {
	typ, _ := fieldGoType(p.GeneratedFile, field)
	if typ[0] == '[' && typ[1] == ']' {
		typ = typ[2:]
	}
	if typ[0] == '*' {
		typ = typ[1:]
	}
	return typ
}

func (p *vtproto) field(field *protogen.Field, fieldname string, proto3 bool) {
	repeated := field.Desc.Cardinality() == protoreflect.Repeated
	typ := p.noStarOrSliceType(field)
	oneof := field.Desc.ContainingOneof() != nil
	switch field.Desc.Kind() {
	case protoreflect.DoubleKind:
		p.P(`var v uint64`)
		p.decodeFixed64("v", "uint64")
		if oneof {
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{`, typ, "(", p.Ident("math", `Float64frombits`), `(v))}`)
		} else if repeated {
			p.P(`v2 := `, typ, "(", p.Ident("math", "Float64frombits"), `(v))`)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v2)`)
		} else if proto3 {
			p.P(`m.`, fieldname, ` = `, typ, "(", p.Ident("math", "Float64frombits"), `(v))`)
		} else {
			p.P(`v2 := `, typ, "(", p.Ident("math", "Float64frombits"), `(v))`)
			p.P(`m.`, fieldname, ` = &v2`)
		}
	case protoreflect.FloatKind:
		p.P(`var v uint32`)
		p.decodeFixed32("v", "uint32")
		if oneof {
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{`, typ, "(", p.Ident("math", "Float32frombits"), `(v))}`)
		} else if repeated {
			p.P(`v2 := `, typ, "(", p.Ident("math", "Float32frombits"), `(v))`)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v2)`)
		} else if proto3 {
			p.P(`m.`, fieldname, ` = `, typ, "(", p.Ident("math", "Float32frombits"), `(v))`)
		} else {
			p.P(`v2 := `, typ, "(", p.Ident("math", "Float32frombits"), `(v))`)
			p.P(`m.`, fieldname, ` = &v2`)
		}
	case protoreflect.Int64Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeVarint("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Uint64Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeVarint("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Int32Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeVarint("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Fixed64Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeFixed64("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeFixed64("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeFixed64("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeFixed64("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Fixed32Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeFixed32("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeFixed32("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeFixed32("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeFixed32("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.BoolKind:
		p.P(`var v int`)
		p.decodeVarint("v", "int")
		if oneof {
			p.P(`b := `, typ, `(v != 0)`)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{b}`)
		} else if repeated {
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, `, typ, `(v != 0))`)
		} else if proto3 {
			p.P(`m.`, fieldname, ` = `, typ, `(v != 0)`)
		} else {
			p.P(`b := `, typ, `(v != 0)`)
			p.P(`m.`, fieldname, ` = &b`)
		}
	case protoreflect.StringKind:
		p.P(`var stringLen uint64`)
		p.decodeVarint("stringLen", "uint64")
		p.P(`intStringLen := int(stringLen)`)
		p.P(`if intStringLen < 0 {`)
		p.P(`return ErrInvalidLength` + p.localName)
		p.P(`}`)
		p.P(`postIndex := iNdEx + intStringLen`)
		p.P(`if postIndex < 0 {`)
		p.P(`return ErrInvalidLength` + p.localName)
		p.P(`}`)
		p.P(`if postIndex > l {`)
		p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
		p.P(`}`)
		if oneof {
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{`, typ, `(dAtA[iNdEx:postIndex])}`)
		} else if repeated {
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, `, typ, `(dAtA[iNdEx:postIndex]))`)
		} else if proto3 {
			p.P(`m.`, fieldname, ` = `, typ, `(dAtA[iNdEx:postIndex])`)
		} else {
			p.P(`s := `, typ, `(dAtA[iNdEx:postIndex])`)
			p.P(`m.`, fieldname, ` = &s`)
		}
		p.P(`iNdEx = postIndex`)
	case protoreflect.GroupKind:
		panic(fmt.Errorf("unmarshaler does not support group %v", fieldname))
	case protoreflect.MessageKind:
		p.P(`var msglen int`)
		p.decodeVarint("msglen", "int")
		p.P(`if msglen < 0 {`)
		p.P(`return ErrInvalidLength` + p.localName)
		p.P(`}`)
		p.P(`postIndex := iNdEx + msglen`)
		p.P(`if postIndex < 0 {`)
		p.P(`return ErrInvalidLength` + p.localName)
		p.P(`}`)
		p.P(`if postIndex > l {`)
		p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
		p.P(`}`)
		if oneof {
			buf := `dAtA[iNdEx:postIndex]`
			p.P(`v := &`, p.noStarOrSliceType(field), `{}`)
			log.Printf("oneof message: %v", field.GoIdent)
			p.decodemessage("v", buf, field.Message.Desc)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{v}`)
		} else if field.Desc.IsMap() {
			goTyp, _ := fieldGoType(p.GeneratedFile, field)
			goTypK, _ := fieldGoType(p.GeneratedFile, field.Message.Fields[0])
			goTypV, _ := fieldGoType(p.GeneratedFile, field.Message.Fields[1])

			p.P(`if m.`, fieldname, ` == nil {`)
			p.P(`m.`, fieldname, ` = make(`, goTyp, `)`)
			p.P(`}`)

			p.P("var mapkey ", goTypK)
			p.P("var mapvalue ", goTypV)
			p.P(`for iNdEx < postIndex {`)

			p.P(`entryPreIndex := iNdEx`)
			p.P(`var wire uint64`)
			p.decodeVarint("wire", "uint64")
			p.P(`fieldNum := int32(wire >> 3)`)

			p.P(`if fieldNum == 1 {`)
			p.mapField("mapkey", field.Message.Fields[0])
			p.P(`} else if fieldNum == 2 {`)
			p.mapField("mapvalue", field.Message.Fields[1])
			p.P(`} else {`)
			p.P(`iNdEx = entryPreIndex`)
			p.P(`skippy, err := skip`, p.localName, `(dAtA[iNdEx:])`)
			p.P(`if err != nil {`)
			p.P(`return err`)
			p.P(`}`)
			p.P(`if (skippy < 0) || (iNdEx + skippy) < 0 {`)
			p.P(`return ErrInvalidLength`, p.localName)
			p.P(`}`)
			p.P(`if (iNdEx + skippy) > postIndex {`)
			p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
			p.P(`}`)
			p.P(`iNdEx += skippy`)
			p.P(`}`)
			p.P(`}`)
			p.P(`m.`, fieldname, `[mapkey] = mapvalue`)
		} else if repeated {
			if p.shouldPool(field.Message) {
				p.P(`v := `, field.Message.GoIdent, `FromVTPool()`)
			} else {
				p.P(`v := &`, p.noStarOrSliceType(field), `{}`)
			}
			buf := `dAtA[iNdEx:postIndex]`
			p.decodemessage("v", buf, field.Message.Desc)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else {
			p.P(`if m.`, fieldname, ` == nil {`)
			if p.shouldPool(field.Message) {
				p.P(`m.`, fieldname, ` = `, field.Message.GoIdent, `FromVTPool()`)
			} else {
				p.P(`m.`, fieldname, ` = &`, p.noStarOrSliceType(field), `{}`)
			}
			p.P(`}`)
			p.decodemessage("m."+fieldname, "dAtA[iNdEx:postIndex]", field.Message.Desc)
		}
		p.P(`iNdEx = postIndex`)

	case protoreflect.BytesKind:
		p.P(`var byteLen int`)
		p.decodeVarint("byteLen", "int")
		p.P(`if byteLen < 0 {`)
		p.P(`return ErrInvalidLength` + p.localName)
		p.P(`}`)
		p.P(`postIndex := iNdEx + byteLen`)
		p.P(`if postIndex < 0 {`)
		p.P(`return ErrInvalidLength` + p.localName)
		p.P(`}`)
		p.P(`if postIndex > l {`)
		p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
		p.P(`}`)
		if oneof {
			p.P(`v := make([]byte, postIndex-iNdEx)`)
			p.P(`copy(v, dAtA[iNdEx:postIndex])`)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{v}`)
		} else if repeated {
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, make([]byte, postIndex-iNdEx))`)
			p.P(`copy(m.`, fieldname, `[len(m.`, fieldname, `)-1], dAtA[iNdEx:postIndex])`)
		} else {
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `[:0] , dAtA[iNdEx:postIndex]...)`)
			p.P(`if m.`, fieldname, ` == nil {`)
			p.P(`m.`, fieldname, ` = []byte{}`)
			p.P(`}`)
		}
		p.P(`iNdEx = postIndex`)
	case protoreflect.Uint32Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeVarint("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.EnumKind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeVarint("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Sfixed32Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeFixed32("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeFixed32("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeFixed32("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeFixed32("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Sfixed64Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeFixed64("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeFixed64("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeFixed64("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeFixed64("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Sint32Kind:
		p.P(`var v `, typ)
		p.decodeVarint("v", typ)
		p.P(`v = `, typ, `((uint32(v) >> 1) ^ uint32(((v&1)<<31)>>31))`)
		if oneof {
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{v}`)
		} else if repeated {
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 {
			p.P(`m.`, fieldname, ` = v`)
		} else {
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Sint64Kind:
		p.P(`var v uint64`)
		p.decodeVarint("v", "uint64")
		p.P(`v = (v >> 1) ^ uint64((int64(v&1)<<63)>>63)`)
		if oneof {
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{`, typ, `(v)}`)
		} else if repeated {
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, `, typ, `(v))`)
		} else if proto3 {
			p.P(`m.`, fieldname, ` = `, typ, `(v)`)
		} else {
			p.P(`v2 := `, typ, `(v)`)
			p.P(`m.`, fieldname, ` = &v2`)
		}
	default:
		panic("not implemented")
	}
}

func (p *vtproto) Ident(path, ident string) string {
	return p.QualifiedGoIdent(protogen.GoImportPath(path).Ident(ident))
}

const ProtoPkg = "google.golang.org/protobuf/proto"

var wireTypes = map[protoreflect.Kind]protowire.Type{
	protoreflect.BoolKind:     protowire.VarintType,
	protoreflect.EnumKind:     protowire.VarintType,
	protoreflect.Int32Kind:    protowire.VarintType,
	protoreflect.Sint32Kind:   protowire.VarintType,
	protoreflect.Uint32Kind:   protowire.VarintType,
	protoreflect.Int64Kind:    protowire.VarintType,
	protoreflect.Sint64Kind:   protowire.VarintType,
	protoreflect.Uint64Kind:   protowire.VarintType,
	protoreflect.Sfixed32Kind: protowire.Fixed32Type,
	protoreflect.Fixed32Kind:  protowire.Fixed32Type,
	protoreflect.FloatKind:    protowire.Fixed32Type,
	protoreflect.Sfixed64Kind: protowire.Fixed64Type,
	protoreflect.Fixed64Kind:  protowire.Fixed64Type,
	protoreflect.DoubleKind:   protowire.Fixed64Type,
	protoreflect.StringKind:   protowire.BytesType,
	protoreflect.BytesKind:    protowire.BytesType,
	protoreflect.MessageKind:  protowire.BytesType,
	protoreflect.GroupKind:    protowire.StartGroupType,
}

func (p *vtproto) unmarshalField(field *protogen.Field, proto3 bool, required protoreflect.FieldNumbers) {
	fieldname := field.GoName
	errFieldname := fieldname
	if field.Oneof != nil {
		fieldname = field.Oneof.GoName
	}

	p.P(`case `, strconv.Itoa(int(field.Desc.Number())), `:`)
	wireType := wireTypes[field.Desc.Kind()]
	if field.Desc.IsList() {
		p.P(`if wireType == `, strconv.Itoa(int(wireType)), `{`)
		p.field(field, fieldname, false)
		p.P(`} else if wireType == `, strconv.Itoa(int(protowire.BytesType)), `{`)
		p.P(`var packedLen int`)
		p.decodeVarint("packedLen", "int")
		p.P(`if packedLen < 0 {`)
		p.P(`return ErrInvalidLength` + p.localName)
		p.P(`}`)
		p.P(`postIndex := iNdEx + packedLen`)
		p.P(`if postIndex < 0 {`)
		p.P(`return ErrInvalidLength` + p.localName)
		p.P(`}`)
		p.P(`if postIndex > l {`)
		p.P(`return `, p.Ident("io", "ErrUnexpectedEOF"))
		p.P(`}`)

		p.P(`var elementCount int`)
		switch field.Desc.Kind() {
		case protoreflect.DoubleKind, protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind:
			p.P(`elementCount = packedLen/`, 8)
		case protoreflect.FloatKind, protoreflect.Fixed32Kind, protoreflect.Sfixed32Kind:
			p.P(`elementCount = packedLen/`, 4)
		case protoreflect.Int64Kind, protoreflect.Uint64Kind, protoreflect.Int32Kind, protoreflect.Uint32Kind, protoreflect.Sint32Kind, protoreflect.Sint64Kind:
			p.P(`var count int`)
			p.P(`for _, integer := range dAtA[iNdEx:postIndex] {`)
			p.P(`if integer < 128 {`)
			p.P(`count++`)
			p.P(`}`)
			p.P(`}`)
			p.P(`elementCount = count`)
		case protoreflect.BoolKind:
			p.P(`elementCount = packedLen`)
		}
		p.P(`if cap(m.`, fieldname, `) < elementCount {`)
		fieldtyp, _ := fieldGoType(p.GeneratedFile, field)
		p.P(`m.`, fieldname, ` = make(`, fieldtyp, `, 0, elementCount)`)
		p.P(`}`)

		p.P(`for iNdEx < postIndex {`)
		p.field(field, fieldname, false)
		p.P(`}`)
		p.P(`} else {`)
		p.P(`return `, p.Ident("fmt", "Errorf"), `("proto: wrong wireType = %d for field `, errFieldname, `", wireType)`)
		p.P(`}`)
	} else {
		p.P(`if wireType != `, strconv.Itoa(int(wireType)), `{`)
		p.P(`return `, p.Ident("fmt", "Errorf"), `("proto: wrong wireType = %d for field `, errFieldname, `", wireType)`)
		p.P(`}`)
		p.field(field, fieldname, proto3)
	}

	if field.Desc.Cardinality() == protoreflect.Required {
		var fieldBit int
		for fieldBit = 0; fieldBit < required.Len(); fieldBit++ {
			if required.Get(fieldBit) == field.Desc.Number() {
				break
			}
		}
		if fieldBit == required.Len() {
			panic("missing required field")
		}
		p.P(`hasFields[`, strconv.Itoa(int(fieldBit/64)), `] |= uint64(`, fmt.Sprintf("0x%08x", uint64(1)<<(fieldBit%64)), `)`)
	}
}

func (p *vtproto) generateMessageUnmarshal(message *protogen.Message, proto3 bool) {
	ccTypeName := message.GoIdent
	required := message.Desc.RequiredNumbers()

	p.P(`func (m *`, ccTypeName, `) UnmarshalVT(dAtA []byte) error {`)
	if required.Len() > 0 {
		p.P(`var hasFields [`, strconv.Itoa(1+(required.Len()-1)/64), `]uint64`)
	}
	p.P(`l := len(dAtA)`)
	p.P(`iNdEx := 0`)
	p.P(`for iNdEx < l {`)
	p.P(`preIndex := iNdEx`)
	p.P(`var wire uint64`)
	p.decodeVarint("wire", "uint64")
	p.P(`fieldNum := int32(wire >> 3)`)
	p.P(`wireType := int(wire & 0x7)`)
	p.P(`if wireType == `, strconv.Itoa(int(protowire.EndGroupType)), ` {`)
	p.P(`return `, p.Ident("fmt", "Errorf"), `("proto: `, message.GoIdent.GoName, `: wiretype end group for non-group")`)
	p.P(`}`)
	p.P(`if fieldNum <= 0 {`)
	p.P(`return `, p.Ident("fmt", "Errorf"), `("proto: `, message.GoIdent.GoName, `: illegal tag %d (wire type %d)", fieldNum, wire)`)
	p.P(`}`)
	p.P(`switch fieldNum {`)
	for _, field := range message.Fields {
		p.unmarshalField(field, proto3, required)
	}
	p.P(`default:`)
	if len(message.Extensions) > 0 {
		c := []string{}
		eranges := message.Desc.ExtensionRanges()
		for e := 0; e < eranges.Len(); e++ {
			erange := eranges.Get(e)
			c = append(c, `((fieldNum >= `, strconv.Itoa(int(erange[0])), ") && (fieldNum < ", strconv.Itoa(int(erange[1])), `))`)
		}
		p.P(`if `, strings.Join(c, "||"), `{`)
		p.P(`var sizeOfWire int`)
		p.P(`for {`)
		p.P(`sizeOfWire++`)
		p.P(`wire >>= 7`)
		p.P(`if wire == 0 {`)
		p.P(`break`)
		p.P(`}`)
		p.P(`}`)
		p.P(`iNdEx-=sizeOfWire`)
		p.P(`skippy, err := skip`, p.localName+`(dAtA[iNdEx:])`)
		p.P(`if err != nil {`)
		p.P(`return err`)
		p.P(`}`)
		p.P(`if (skippy < 0) || (iNdEx + skippy) < 0 {`)
		p.P(`return ErrInvalidLength`, p.localName)
		p.P(`}`)
		p.P(`if (iNdEx + skippy) > l {`)
		p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
		p.P(`}`)
		p.P(p.Ident(ProtoPkg, "AppendExtension"), `(m, int32(fieldNum), dAtA[iNdEx:iNdEx+skippy])`)
		p.P(`iNdEx += skippy`)
		p.P(`} else {`)
	}
	p.P(`iNdEx=preIndex`)
	p.P(`skippy, err := skip`, p.localName, `(dAtA[iNdEx:])`)
	p.P(`if err != nil {`)
	p.P(`return err`)
	p.P(`}`)
	p.P(`if (skippy < 0) || (iNdEx + skippy) < 0 {`)
	p.P(`return ErrInvalidLength`, p.localName)
	p.P(`}`)
	p.P(`if (iNdEx + skippy) > l {`)
	p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
	p.P(`}`)
	// TODO@vmg migrate eventually
	p.P(`m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)`)
	p.P(`iNdEx += skippy`)
	if len(message.Extensions) > 0 {
		p.P(`}`)
	}
	p.P(`}`)
	p.P(`}`)

	for _, field := range message.Fields {
		if field.Desc.Cardinality() != protoreflect.Required {
			continue
		}
		var fieldBit int
		for fieldBit = 0; fieldBit < required.Len(); fieldBit++ {
			if required.Get(fieldBit) == field.Desc.Number() {
				break
			}
		}
		if fieldBit == required.Len() {
			panic("missing required field")
		}
		p.P(`if hasFields[`, strconv.Itoa(int(fieldBit/64)), `] & uint64(`, fmt.Sprintf("0x%08x", uint64(1)<<(fieldBit%64)), `) == 0 {`)
		p.P(`return new(`, p.Ident(ProtoPkg, "RequiredNotSetError"), `)`)
		p.P(`}`)
	}
	p.P()
	p.P(`if iNdEx > l {`)
	p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
	p.P(`}`)
	p.P(`return nil`)
	p.P(`}`)
}

func (p *vtproto) generateUnmarshalHelpers() {
	p.P(`func skip`, p.localName, `(dAtA []byte) (n int, err error) {
		l := len(dAtA)
		iNdEx := 0
		depth := 0
		for iNdEx < l {
			var wire uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflow`, p.localName, `
				}
				if iNdEx >= l {
					return 0, `, p.Ident("io", "ErrUnexpectedEOF"), `
				}
				b := dAtA[iNdEx]
				iNdEx++
				wire |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			wireType := int(wire & 0x7)
			switch wireType {
			case 0:
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflow`, p.localName, `
					}
					if iNdEx >= l {
						return 0, `, p.Ident("io", "ErrUnexpectedEOF"), `
					}
					iNdEx++
					if dAtA[iNdEx-1] < 0x80 {
						break
					}
				}
			case 1:
				iNdEx += 8
			case 2:
				var length int
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflow`, p.localName, `
					}
					if iNdEx >= l {
						return 0, `, p.Ident("io", "ErrUnexpectedEOF"), `
					}
					b := dAtA[iNdEx]
					iNdEx++
					length |= (int(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				if length < 0 {
					return 0, ErrInvalidLength`, p.localName, `
				}
				iNdEx += length
			case 3:
				depth++
			case 4:
				if depth == 0 {
					return 0, ErrUnexpectedEndOfGroup`, p.localName, `
				}
				depth--
			case 5:
				iNdEx += 4
			default:
				return 0, `, p.Ident("fmt", `Errorf`), `("proto: illegal wireType %d", wireType)
			}
			if iNdEx < 0 {
				return 0, ErrInvalidLength`, p.localName, `
			}
			if depth == 0 {
				return iNdEx, nil
			}
		}
		return 0, `, p.Ident("io", "ErrUnexpectedEOF"), `
	}

	var (
		ErrInvalidLength`, p.localName, ` = `, p.Ident("fmt", "Errorf"), `("proto: negative length found during unmarshaling")
		ErrIntOverflow`, p.localName, ` = `, p.Ident("fmt", "Errorf"), `("proto: integer overflow")
		ErrUnexpectedEndOfGroup`, p.localName, ` = `, p.Ident("fmt", "Errorf"), `("proto: unexpected end of group")
	)
	`)
}