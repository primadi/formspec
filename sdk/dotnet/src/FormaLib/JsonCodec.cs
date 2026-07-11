using System.Text;

namespace Forma;

/// <summary>
/// Minimal JSON codec — stdlib only, no System.Text.Json or Newtonsoft dependency.
/// Supports the subset of JSON used in the forma-sidecar wire protocol.
/// For production use, swap this out for System.Text.Json.
/// </summary>
internal static class JsonCodec
{
    public static string Encode(object? obj)
    {
        var sb = new StringBuilder();
        EncodeValue(sb, obj);
        return sb.ToString();
    }

    public static Dictionary<string, object?>? Decode(string json)
    {
        if (string.IsNullOrWhiteSpace(json)) return null;
        var parser = new JsonParser(json);
        var result = parser.ParseValue();
        return result as Dictionary<string, object?>;
    }

    private static void EncodeValue(StringBuilder sb, object? val)
    {
        if (val is null)
        {
            sb.Append("null");
        }
        else if (val is Dictionary<string, object?> dict)
        {
            sb.Append('{');
            bool first = true;
            foreach (var kv in dict)
            {
                if (!first) sb.Append(',');
                first = false;
                EncodeString(sb, kv.Key);
                sb.Append(':');
                EncodeValue(sb, kv.Value);
            }
            sb.Append('}');
        }
        else if (val is List<object?> list)
        {
            sb.Append('[');
            for (int i = 0; i < list.Count; i++)
            {
                if (i > 0) sb.Append(',');
                EncodeValue(sb, list[i]);
            }
            sb.Append(']');
        }
        else if (val is string s)
        {
            EncodeString(sb, s);
        }
        else if (val is bool b)
        {
            sb.Append(b ? "true" : "false");
        }
        else if (val is int || val is long || val is short || val is byte)
        {
            sb.Append(Convert.ToInt64(val));
        }
        else if (val is float || val is double || val is decimal)
        {
            sb.Append(Convert.ToDouble(val).ToString("G"));
        }
        else
        {
            EncodeString(sb, val.ToString() ?? "");
        }
    }

    private static void EncodeString(StringBuilder sb, string s)
    {
        sb.Append('"');
        foreach (var c in s)
        {
            switch (c)
            {
                case '"': sb.Append("\\\""); break;
                case '\\': sb.Append("\\\\"); break;
                case '\b': sb.Append("\\b"); break;
                case '\f': sb.Append("\\f"); break;
                case '\n': sb.Append("\\n"); break;
                case '\r': sb.Append("\\r"); break;
                case '\t': sb.Append("\\t"); break;
                default:
                    if (c < 0x20)
                        sb.Append($"\\u{(int)c:x4}");
                    else
                        sb.Append(c);
                    break;
            }
        }
        sb.Append('"');
    }

    private sealed class JsonParser
    {
        private readonly string _json;
        private int _pos;

        public JsonParser(string json) { _json = json; _pos = 0; }

        private char Peek() => _pos < _json.Length ? _json[_pos] : '\0';
        private char Next() => _pos < _json.Length ? _json[_pos++] : '\0';
        private void SkipWs()
        {
            while (_pos < _json.Length && _json[_pos] <= ' ') _pos++;
        }

        public object? ParseValue()
        {
            SkipWs();
            return Peek() switch
            {
                '{' => ParseObject(),
                '[' => ParseArray(),
                '"' => ParseString(),
                't' or 'f' => ParseBool(),
                'n' => { Expect("null"); return null; },
                _ => ParseNumber()
            };
        }

        private Dictionary<string, object?> ParseObject()
        {
            var dict = new Dictionary<string, object?>();
            Next(); // '{'
            SkipWs();
            if (Peek() == '}') { Next(); return dict; }
            while (true)
            {
                SkipWs();
                var key = ParseString();
                SkipWs();
                if (Next() != ':') throw new FormaException("expected ':'");
                dict[key] = ParseValue();
                SkipWs();
                if (Peek() == '}') { Next(); return dict; }
                if (Next() != ',') throw new FormaException("expected ',' or '}'");
            }
        }

        private List<object?> ParseArray()
        {
            var list = new List<object?>();
            Next(); // '['
            SkipWs();
            if (Peek() == ']') { Next(); return list; }
            while (true)
            {
                list.Add(ParseValue());
                SkipWs();
                if (Peek() == ']') { Next(); return list; }
                if (Next() != ',') throw new FormaException("expected ',' or ']'");
            }
        }

        private string ParseString()
        {
            var sb = new StringBuilder();
            Next(); // '"'
            while (_pos < _json.Length && _json[_pos] != '"')
            {
                if (_json[_pos] == '\\')
                {
                    _pos++;
                    switch (Next())
                    {
                        case '"': sb.Append('"'); break;
                        case '\\': sb.Append('\\'); break;
                        case '/': sb.Append('/'); break;
                        case 'b': sb.Append('\b'); break;
                        case 'f': sb.Append('\f'); break;
                        case 'n': sb.Append('\n'); break;
                        case 'r': sb.Append('\r'); break;
                        case 't': sb.Append('\t'); break;
                        case 'u':
                            var hex = _json.Substring(_pos, 4);
                            _pos += 4;
                            sb.Append((char)Convert.ToInt32(hex, 16));
                            break;
                    }
                }
                else
                {
                    sb.Append(Next());
                }
            }
            if (_pos < _json.Length) Next(); // '"'
            return sb.ToString();
        }

        private object ParseNumber()
        {
            var start = _pos;
            if (Peek() == '-') _pos++;
            while (_pos < _json.Length && char.IsDigit(_json[_pos])) _pos++;
            var isDecimal = false;
            if (_pos < _json.Length && _json[_pos] == '.')
            {
                isDecimal = true;
                _pos++;
                while (_pos < _json.Length && char.IsDigit(_json[_pos])) _pos++;
            }
            if (_pos < _json.Length && (_json[_pos] == 'e' || _json[_pos] == 'E'))
            {
                isDecimal = true;
                _pos++;
                if (_pos < _json.Length && (_json[_pos] == '+' || _json[_pos] == '-')) _pos++;
                while (_pos < _json.Length && char.IsDigit(_json[_pos])) _pos++;
            }
            var str = _json[start.._pos];
            if (isDecimal) return double.Parse(str);
            var val = long.Parse(str);
            if (val >= int.MinValue && val <= int.MaxValue) return (int)val;
            return val;
        }

        private bool ParseBool()
        {
            if (Peek() == 't') { Expect("true"); return true; }
            Expect("false"); return false;
        }

        private void Expect(string s)
        {
            foreach (var c in s)
            {
                if (Next() != c) throw new FormaException($"expected '{s}'");
            }
        }
    }
}
