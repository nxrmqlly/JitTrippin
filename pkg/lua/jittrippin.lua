---@meta

---@class PipelineConfig
---@field description? string
---@field visibility? "public" | "private"

---@class CheckoutConfig
---@field url string
---@field branch string

---@class JobConfig
---@field image? string
---@field env? table<string, string>

---@class StepConfig
---@field cmd string

---@class NeedsConfig
---@field requires? string[]

---@class GitHubPushConfig
---@field branch? string | string[]
---@field tag? string | string[]

---@class GitHubReleaseArtifact
---@field job string
---@field name string
---@field as? string

---@class GitHubReleaseConfig
---@field on string
---@field artifacts? GitHubReleaseArtifact[]

---@class GitHubConfig
---@field push? GitHubPushConfig
---@field release? GitHubReleaseConfig

---@param name string
---@param config? PipelineConfig
function pipeline(name, config) end

---@param config CheckoutConfig
function checkout(config) end

---@param name string
---@param config? JobConfig
function job(name, config) end

---@overload fun(name: string, config: StepConfig)
---@param command string
function run(command) end

---@param ... string
function needs(...) end

---@param config NeedsConfig
function needs(config) end

---@param name string
---@param path string
function artifact(name, path) end

---@param config GitHubConfig
function github(config) end
