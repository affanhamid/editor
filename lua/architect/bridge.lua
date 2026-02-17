local M = {}

function M.connect(socket_path, on_event)
  local pipe = vim.uv.new_pipe(false)
  pipe:connect(socket_path, function(err)
    if err then
      vim.schedule(function()
        vim.notify("architect-bridge: connection failed: " .. err, vim.log.levels.ERROR)
      end)
      return
    end

    pipe:read_start(function(read_err, data)
      if read_err then
        vim.schedule(function()
          vim.notify("architect-bridge: read error: " .. read_err, vim.log.levels.ERROR)
        end)
        return
      end
      if data then
        for line in data:gmatch("[^\n]+") do
          local ok, event = pcall(vim.json.decode, line)
          if ok then
            vim.schedule(function() on_event(event) end)
          end
        end
      end
    end)
  end)

  M.pipe = pipe
  return M
end

function M.send(command)
  if M.pipe then
    M.pipe:write(vim.json.encode(command) .. "\n")
  end
end

function M.disconnect()
  if M.pipe then
    M.pipe:read_stop()
    M.pipe:close()
    M.pipe = nil
  end
end

return M
