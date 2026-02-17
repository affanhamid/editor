local M = {}

function M.setup(opts)
  opts = opts or {}
  M.config = {
    socket_path = opts.socket_path or "/tmp/architect-bridge.sock",
    bridge_binary = opts.bridge_binary,
    db_url = opts.db_url or "postgres://architect:architect_local@localhost:5432/architect?sslmode=disable",
  }

  local bridge = require("architect.bridge")
  local swarm = require("architect.ui.swarm_panel")
  local chat = require("architect.ui.chat_room")
  local dag = require("architect.ui.dag_view")
  local prompt = require("architect.ui.prompt_input")
  local consultation = require("architect.ui.consultation")

  -- Start the bridge binary if configured
  if M.config.bridge_binary then
    M.bridge_job = vim.fn.jobstart({
      M.config.bridge_binary,
      "--db", M.config.db_url,
      "--socket", M.config.socket_path,
    }, { detach = true })
    -- Give it a moment to start
    vim.defer_fn(function() M.connect_bridge(bridge, swarm, chat, dag, consultation) end, 500)
  else
    M.connect_bridge(bridge, swarm, chat, dag, consultation)
  end

  -- Keymaps
  vim.keymap.set("n", "<leader>as", function()
    swarm.create():mount()
    bridge.send({ type = "request_snapshot" })
  end, { desc = "Architect: Swarm Panel" })

  vim.keymap.set("n", "<leader>ac", function()
    chat.create():mount()
    bridge.send({ type = "request_snapshot" })
  end, { desc = "Architect: Chat Room" })

  vim.keymap.set("n", "<leader>ad", function()
    M.pending_dag = true
    bridge.send({ type = "request_snapshot" })
  end, { desc = "Architect: DAG View" })

  vim.keymap.set("n", "<leader>ap", function()
    prompt.show(bridge)
  end, { desc = "Architect: Submit Prompt" })
end

function M.connect_bridge(bridge, swarm, chat, dag, consultation)
  bridge.connect(M.config.socket_path, function(event)
    if event.type == "snapshot" then
      swarm.update(event.data.agents or {})
      for _, msg in ipairs(event.data.messages or {}) do
        chat.append(msg)
      end
      if M.pending_dag then
        dag.show(event.data.tasks or {}, event.data.edges or {})
        M.pending_dag = false
      end

    elseif event.type == "agent_update" then
      swarm.update_single(event.data)

    elseif event.type == "new_message" then
      chat.append(event.data)
      if event.data.msg_type == "blocker" then
        vim.notify(
          string.format("BLOCKER from %s: %s", (event.data.agent_id or ""):sub(1, 8), event.data.content),
          vim.log.levels.WARN
        )
      end

    elseif event.type == "task_update" then
      -- Use event data directly instead of requesting full snapshot

    elseif event.type == "consultation_request" then
      consultation.show(event.data, bridge)
    end
  end)
end

return M
