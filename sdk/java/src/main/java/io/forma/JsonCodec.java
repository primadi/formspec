package io.forma;

import java.io.StringReader;
import java.io.StringWriter;
import java.util.*;

/**
 * Minimal JSON codec — stdlib only, no Jackson/Gson dependency.
 * Supports the subset of JSON used in the forma-sidecar wire protocol.
 * For production use, swap this out for Jackson/Gson.
 */
final class JsonCodec {

    private JsonCodec() {
    }

    /** Encode a Java object to JSON string. */
    static String encode(Object obj) {
        var sw = new StringWriter();
        encodeValue(sw, obj);
        return sw.toString();
    }

    /** Decode a JSON string to a Java Map/List/String/Number/Boolean/null. */
    @SuppressWarnings("unchecked")
    static <T> T decode(String json) {
        if (json == null || json.isBlank()) return null;
        return (T) new JsonDecoder(json).parseValue();
    }

    // ---- Encoder ----

    private static void encodeValue(StringWriter w, Object val) {
        if (val == null) {
            w.write("null");
        } else if (val instanceof Map) {
            encodeMap(w, (Map<String, Object>) val);
        } else if (val instanceof List) {
            encodeList(w, (List<Object>) val);
        } else if (val instanceof String s) {
            w.write('"');
            for (int i = 0; i < s.length(); i++) {
                char c = s.charAt(i);
                switch (c) {
                    case '"' -> w.write("\\\"");
                    case '\\' -> w.write("\\\\");
                    case '\b' -> w.write("\\b");
                    case '\f' -> w.write("\\f");
                    case '\n' -> w.write("\\n");
                    case '\r' -> w.write("\\r");
                    case '\t' -> w.write("\\t");
                    default -> {
                        if (c < 0x20) {
                            w.write(String.format("\\u%04x", (int) c));
                        } else {
                            w.write(c);
                        }
                    }
                }
            }
            w.write('"');
        } else if (val instanceof Boolean) {
            w.write(val.toString());
        } else if (val instanceof Number) {
            w.write(val.toString());
        } else {
            w.write('"');
            w.write(val.toString());
            w.write('"');
        }
    }

    private static void encodeMap(StringWriter w, Map<String, Object> map) {
        w.write('{');
        var first = true;
        for (var entry : map.entrySet()) {
            if (!first) w.write(',');
            first = false;
            encodeValue(w, entry.getKey());
            w.write(':');
            encodeValue(w, entry.getValue());
        }
        w.write('}');
    }

    private static void encodeList(StringWriter w, List<Object> list) {
        w.write('[');
        for (int i = 0; i < list.size(); i++) {
            if (i > 0) w.write(',');
            encodeValue(w, list.get(i));
        }
        w.write(']');
    }

    // ---- Decoder ----

    private static class JsonDecoder {
        private final StringReader reader;
        private int ch;

        JsonDecoder(String json) {
            this.reader = new StringReader(json);
            nextChar();
        }

        private void nextChar() {
            try {
                ch = reader.read();
            } catch (Exception e) {
                ch = -1;
            }
        }

        private void skipWhitespace() {
            while (ch >= 0 && ch <= ' ') {
                nextChar();
            }
        }

        Object parseValue() {
            skipWhitespace();
            return switch (ch) {
                case '{' -> parseObject();
                case '[' -> parseArray();
                case '"' -> parseString();
                case 't', 'f' -> parseBoolean();
                case 'n' -> parseNull();
                case -1 -> null;
                default -> parseNumber();
            };
        }

        Map<String, Object> parseObject() {
            var map = new LinkedHashMap<String, Object>();
            nextChar(); // consume '{'
            skipWhitespace();
            if (ch == '}') {
                nextChar();
                return map;
            }
            while (true) {
                skipWhitespace();
                if (ch != '"') throw new FormaException("expected string key in JSON object");
                var key = parseString();
                skipWhitespace();
                if (ch != ':') throw new FormaException("expected ':' in JSON object");
                nextChar();
                var val = parseValue();
                map.put(key, val);
                skipWhitespace();
                if (ch == '}') {
                    nextChar();
                    return map;
                }
                if (ch != ',') throw new FormaException("expected ',' or '}' in JSON object");
                nextChar();
            }
        }

        List<Object> parseArray() {
            var list = new ArrayList<Object>();
            nextChar(); // consume '['
            skipWhitespace();
            if (ch == ']') {
                nextChar();
                return list;
            }
            while (true) {
                list.add(parseValue());
                skipWhitespace();
                if (ch == ']') {
                    nextChar();
                    return list;
                }
                if (ch != ',') throw new FormaException("expected ',' or ']' in JSON array");
                nextChar();
            }
        }

        String parseString() {
            var sb = new StringBuilder();
            nextChar(); // consume opening '"'
            while (ch >= 0 && ch != '"') {
                if (ch == '\\') {
                    nextChar();
                    switch (ch) {
                        case '"' -> sb.append('"');
                        case '\\' -> sb.append('\\');
                        case '/' -> sb.append('/');
                        case 'b' -> sb.append('\b');
                        case 'f' -> sb.append('\f');
                        case 'n' -> sb.append('\n');
                        case 'r' -> sb.append('\r');
                        case 't' -> sb.append('\t');
                        case 'u' -> {
                            var hex = new char[4];
                            for (int i = 0; i < 4; i++) nextChar();
                            // simplified: read 4 hex digits already consumed
                            sb.append((char) Integer.parseInt(new String(hex), 16));
                        }
                        default -> sb.append((char) ch);
                    }
                } else {
                    sb.append((char) ch);
                }
                nextChar();
            }
            if (ch == '"') nextChar(); // consume closing '"'
            return sb.toString();
        }

        Number parseNumber() {
            var sb = new StringBuilder();
            if (ch == '-') {
                sb.append('-');
                nextChar();
            }
            while (ch >= '0' && ch <= '9') {
                sb.append((char) ch);
                nextChar();
            }
            boolean isDecimal = false;
            if (ch == '.') {
                isDecimal = true;
                sb.append('.');
                nextChar();
                while (ch >= '0' && ch <= '9') {
                    sb.append((char) ch);
                    nextChar();
                }
            }
            if (ch == 'e' || ch == 'E') {
                isDecimal = true;
                sb.append((char) ch);
                nextChar();
                if (ch == '+' || ch == '-') {
                    sb.append((char) ch);
                    nextChar();
                }
                while (ch >= '0' && ch <= '9') {
                    sb.append((char) ch);
                    nextChar();
                }
            }
            var str = sb.toString();
            if (isDecimal) {
                return Double.parseDouble(str);
            }
            var val = Long.parseLong(str);
            if (val >= Integer.MIN_VALUE && val <= Integer.MAX_VALUE) {
                return (int) val;
            }
            return val;
        }

        boolean parseBoolean() {
            if (ch == 't') {
                expect("true");
                return true;
            }
            expect("false");
            return false;
        }

        Object parseNull() {
            expect("null");
            return null;
        }

        private void expect(String literal) {
            for (int i = 0; i < literal.length(); i++) {
                if (ch != literal.charAt(i)) {
                    throw new FormaException("expected '" + literal + "'");
                }
                nextChar();
            }
        }
    }
}
