require "json"
require "socket"

module FormSpec
  # HTTP client to the formspec-sidecar local listener — unix domain socket
  # (default) or localhost TCP.
  class SidecarClient
    # @param endpoint [String] "unix:///tmp/formspec/sidecar.sock" or "http://localhost:PORT"
    # @param timeout  [Integer] request timeout in seconds
    def initialize(endpoint = nil, timeout: 30)
      endpoint ||= ENV["FORMA_SIDECAR_SOCKET"] || "unix:///tmp/formspec/sidecar.sock"
      @timeout = timeout

      if endpoint.start_with?("unix://")
        @socket_path = endpoint.sub(/\Aunix:\/\//, "")
        @tcp_host = nil
        @tcp_port = nil
      elsif endpoint.start_with?("http://")
        @socket_path = nil
        uri = URI.parse(endpoint)
        @tcp_host = uri.host
        @tcp_port = uri.port || 80
      else
        raise FormSpecError, "sidecar endpoint #{endpoint}: unsupported scheme (want unix:// or http://)"
      end
    end

    # POST to a sidecar endpoint.
    #
    # @param path [String] request path (e.g., "/ctx/db/query")
    # @param body [Hash]   JSON-serializable request body
    # @return [Hash] decoded JSON response
    def post(path, body)
      json_body = JSON.generate(body)
      request = build_http_request("POST", path, json_body)

      sock = create_socket
      begin
        sock.write(request)
        sock.flush
        sock.close_write
        parse_http_response(sock, path)
      ensure
        sock.close rescue nil
      end
    end

    private

    def create_socket
      if @socket_path
        sock = UNIXSocket.new(@socket_path)
        sock.settimeout(@timeout)
        sock
      else
        sock = TCPSocket.new(@tcp_host, @tcp_port)
        sock.settimeout(@timeout)
        sock
      end
    end

    def build_http_request(method, path, body)
      <<~HTTP
        #{method} #{path} HTTP/1.1\r
        Host: localhost\r
        Content-Type: application/json\r
        Content-Length: #{body.bytesize}\r
        Connection: close\r
        \r
        #{body}
      HTTP
    end

    def parse_http_response(sock, path)
      # Parse status line
      status_line = sock.gets
      raise FormSpecError, "empty response from sidecar" if status_line.nil? || status_line.empty?

      _http_version, status_code, _status_text = status_line.strip.split(" ", 3)
      status_code = status_code.to_i

      # Parse headers
      content_length = nil
      while (line = sock.gets)
        line = line.strip
        break if line.empty?
        if line.downcase.start_with?("content-length:")
          content_length = line.split(":", 2).last.strip.to_i
        end
      end

      # Parse body
      body = if content_length && content_length > 0
               sock.read(content_length)
             else
               sock.read
             end

      decoded = body ? JSON.parse(body) : {}
      unless status_code == 200
        error_msg = decoded.is_a?(Hash) ? decoded["error"] : "HTTP #{status_code}"
        raise FormSpecError, "sidecar call #{path}: #{error_msg}"
      end

      decoded.is_a?(Hash) ? decoded : {}
    rescue JSON::ParserError
      raise FormSpecError, "sidecar call #{path}: invalid JSON response"
    end
  end
end
