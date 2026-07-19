import { api } from '@/lib/api'

export interface GitHubWorkflowRun {
  id: number
  name: string
  display_title: string
  event: string
  status: string
  conclusion: string
  html_url: string
  head_branch: string
  head_sha: string
  run_number: number
  created_at: string
  updated_at: string
}

export interface GitHubRelease {
  id: number
  tag_name: string
  name: string
  html_url: string
  draft: boolean
  prerelease: boolean
  published_at: string
}

export interface GitHubRepository {
  id: number
  full_name: string
  description: string
  default_branch: string
  html_url: string
  private: boolean
  archived: boolean
  updated_at: string
  open_issues_count: number
  stargazers_count: number
  forks_count: number
  size: number
  language: string
  visibility: string
  permissions: { admin: boolean; maintain: boolean; push: boolean; triage: boolean; pull: boolean }
}

export interface GitHubProject {
  repository: GitHubRepository
  workflow_runs: GitHubWorkflowRun[]
  releases: GitHubRelease[]
  error?: string
}

export interface GitHubOverview { generated_at: string; projects: GitHubProject[]; warnings: string[] }

export interface GitHubCommitFile {
  filename: string
  previous_filename?: string
  status: string
  additions: number
  deletions: number
  changes: number
  patch?: string
}

export interface GitHubCommit {
  sha: string
  html_url: string
  commit: { message: string; author: { name: string; email: string; date: string } }
  stats?: { additions: number; deletions: number; total: number }
  files?: GitHubCommitFile[]
}

export interface GitHubWorkflow {
  id: number
  name: string
  path: string
  state: string
  html_url: string
  created_at: string
  updated_at: string
}

export interface GitHubWorkflowFile { name: string; path: string; sha: string; content: string; encoding: string }

export const getGitHubOverview = async () =>
  (await api.get<GitHubOverview>('/v1/github/overview', { timeout: 45_000 })).data

export const runGitHubWorkflowAction = async (repository: string, runID: number, action: 'rerun' | 'cancel') => {
  const [owner, repo] = repository.split('/', 2)
  return (await api.post<{ status: string }>(`/v1/github/repos/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/actions/runs/${runID}/${action}`, {})).data
}

const repositoryPath = (repository: string) => {
  const [owner, repo] = repository.split('/', 2)
  return `${encodeURIComponent(owner)}/${encodeURIComponent(repo)}`
}

export const getGitHubCommits = async (repository: string) =>
  (await api.get<{ items: GitHubCommit[] }>(`/v1/github/repos/${repositoryPath(repository)}/commits`)).data.items

export const getGitHubCommit = async (repository: string, sha: string) =>
  (await api.get<GitHubCommit>(`/v1/github/repos/${repositoryPath(repository)}/commits/${encodeURIComponent(sha)}`)).data

export const getGitHubWorkflows = async (repository: string) =>
  (await api.get<{ items: GitHubWorkflow[] }>(`/v1/github/repos/${repositoryPath(repository)}/workflows`)).data.items

export const runGitHubWorkflow = async (repository: string, workflowID: number, action: 'dispatch' | 'enable' | 'disable', input: { ref?: string; inputs?: Record<string, string> } = {}) =>
  (await api.post<{ status: string }>(`/v1/github/repos/${repositoryPath(repository)}/workflows/${workflowID}/${action}`, input)).data

export const getGitHubWorkflowFile = async (repository: string, path: string, ref = '') =>
  (await api.get<GitHubWorkflowFile>(`/v1/github/repos/${repositoryPath(repository)}/workflow-file`, { params: { path, ref } })).data

export const updateGitHubWorkflowFile = async (repository: string, input: { path: string; branch: string; sha: string; message: string; content: string }) => {
  const bytes = new TextEncoder().encode(input.content)
  let binary = ''
  bytes.forEach((byte) => { binary += String.fromCharCode(byte) })
  return (await api.put<{ status: string }>(`/v1/github/repos/${repositoryPath(repository)}/workflow-file`, {
    path: input.path,
    branch: input.branch,
    sha: input.sha,
    message: input.message,
    content_base64: btoa(binary),
  })).data
}
