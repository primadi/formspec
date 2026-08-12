require "json"
require "socket"

module FormSpec
  # The lib-formspec-ruby listener: accepts POST /invoke/{module}/{entity}/{action}
  # from formspec-sidecar and dispatches to registered handlers. Also answers
  # GET /health for the sidecar's app monitor.
  #
  # @example
  #   app = FormSpec::App.new
  #   app.handle("billing.invoice.approve") do |inv, ctx|
  #     ctx.lock.acquire("invoice:#{inv.resource_id}", 30)
  #     ActionResult.new({ approved_at: Time.now.iso8601 }, new_state: "approved")
  #   end
  #   app.run
  class App
    # @param listen          [String, nil] "unix:///path.sock"; default from FORMA_APP_SOCKET
    # @param sidecar_endpoint [String, nil] "unix:///..." or "http://..."; default from FORMA_SIDECAR_SOCKET
    def initialize(listen: nil, sidecar_endpoint: nil)
      @listen = listen || "unix://#{ENV.fetch("FORMA_APP_SOCKET", "/tmp/formspec/app.sock")}"
      @handlers = {}
      @ctx = Ctx.new(SidecarClient.new(sidecar_endpoint))
    end

    # Register a handler for "module.entity.action".
    #
    # @param action  [String]  "module.entity.action"
    # @param block   [Proc]    handler(Invocation, Ctx) -> ActionResult | Hash | nil
    def handle(action, &block)
      if @handlers.key?(action)
        raise FormSpecError, "handler for #{action} already registered"
      end
      @handlers[action] = block
    end

    # Blocks serving requests until the process is terminated.
    def run
      unless @listen.start_with?("unix://")
        raise FormSpecError, "listen #{@listen}: only unix:// is supported by lib-formspec-ruby"
      end

      socket_path = @listen.sub(/\Aunix:\/\//, "")
      FileUtils.mkdir_p(File.dirname(socket_path))
      File.unlink(socket_path) if File.exist?(socket_path)

      server = UNIXServer.new(socket_path)
      File.chmod(0o666, socket_path)

      $stderr.puts "[lib-formspec-ruby] listening on #{socket_path}"

      loop do
        client = server.accept
        Thread.new(client) { |conn| serve_one(conn) }
      end
    rescue Interrupt
      # graceful shutdown
    ensure
      server&.close
    end

    private

    def serve_one(conn)
      begin
        request_line = conn.gets
        return if request_line.nil? || request_line.empty?

        method, path, _http_ver = request_line.strip.split(" ", 3)

        # Read headers
        content_length = nil
        while (line = conn.gets)
          line = line.strip
          break if line.empty?
          if line.downcase.start_with?("content-length:")
            content_length = line.split(":", 2).last.strip.to_i
          end
        end

        # Read body
        body = if content_length && content_length > 0
                 conn.read(content_length)
               else
                 ""
               end

        handle_request(conn, method, path, body)
      rescue => e
        respond(conn, 500, { "error" => e.message })
      ensure
        conn.close rescue nil
      end
    end

    def handle_request(conn, method, path, body)
      if method == "GET" && path == "/health"
        respond(conn, 200, { "status" => "healthy", "handlers" => @handlers.size })
        return
      end

      unless method == "POST" && path.start_with?("/invoke/")
        respond(conn, 404, { "error" => "expected POST /invoke/{module}/{entity}/{action}" })
        return
      end

      # Parse /invoke/{module}/{entity}/{action}
      segments = path.split("/", 5)
      if segments.length < 5
        respond(conn, 400, { "error" => "invalid invoke path: #{path}" })
        return
      end

      module_name  = segments[2]
      entity_name  = segments[3]
      action_name  = segments[4]
      action_key   = "#{module_name}.#{entity_name}.#{action_name}"

      handler = @handlers[action_key]
      unless handler
        respond(conn, 404, { "error" => "no handler for #{action_key}" })
        return
      end

      body_obj = body.empty? ? {} : JSON.parse(body)
      inv = Invocation.from_request(module_name, entity_name, action_name, body_obj)
      result = handler.call(inv, @ctx)

      wire_result = if result.is_a?(ActionResult)
                      result.to_wire
                    else
                      { "data" => result }
                    end

      respond(conn, 200, wire_result)
    rescue JSON::ParserError
      respond(conn, 400, { "error" => "invalid JSON body" })
    end

    def respond(conn, status, body)
      json = JSON.generate(body)
      conn.write("HTTP/1.1 #{status} #{status == 200 ? "OK" : "Error"}\r\n")
      conn.write("Content-Type: application/json\r\n")
      conn.write("Content-Length: #{json.bytesize}\r\n")
      conn.write("Connection: close\r\n")
      conn.write("\r\n")
      conn.write(json)
      conn.flush
    end
  end
end
