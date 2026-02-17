local NuiSplit = require("nui.split")
local utils = require("architect.utils")

local M = {}
M.agents = {}

function M.create()
  local panel = NuiSplit({
    relative = "editor",
    position = "right",
    size = 40,
    buf_options = {
      modifiable = false,
      filetype = "architect-swarm",
    },
  })
  M.panel = panel
  return panel
end

function M.update_single(agent_data)
  if not agent_data or not agent_data.agent_id then return end
  local found = false
  for i, a in ipairs(M.agents) do
    if a.agent_id == agent_data.agent_id then
      M.agents[i] = agent_data
      found = true
      break
    end
  end
  if not found then
    table.insert(M.agents, agent_data)
  end
  M.update(M.agents)
end

function M.update(agents)
  M.agents = agents
  if not M.panel then return end
  local buf = M.panel.bufnr
  if not buf or not vim.api.nvim_buf_is_valid(buf) then return end
  vim.bo[buf].modifiable = true

  local lines = { "  AGENT SWARM", "  ───────────" }
  for _, agent in ipairs(agents) do
    local icon = utils.status_icons[agent.status] or "?"
    local task_info = agent.current_task_id and (" → task #" .. agent.current_task_id) or ""
    table.insert(lines, string.format("  %s %s%s", icon, utils.short_id(agent.agent_id), task_info))
  end

  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  vim.bo[buf].modifiable = false

  -- Apply highlights
  for i, agent in ipairs(agents) do
    local hl = utils.status_hl[agent.status] or "Normal"
    vim.api.nvim_buf_add_highlight(buf, -1, hl, i + 1, 2, 3)
  end
end

return M
