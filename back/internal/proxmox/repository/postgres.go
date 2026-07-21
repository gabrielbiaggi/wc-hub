package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	adapter "github.com/webcreations/wc-hub/back/internal/adapters/proxmox"
)

type Postgres struct{ db *pgxpool.Pool }
type Summary struct {
	Configured    bool       `json:"configured"`
	Status        string     `json:"status"`
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
	Nodes         int        `json:"nodes"`
	VMs           int        `json:"virtual_machines"`
	Containers    int        `json:"containers"`
	Storage       int        `json:"storage_pools"`
}

func NewPostgres(db *pgxpool.Pool) *Postgres { return &Postgres{db: db} }
func (r *Postgres) StartRun(ctx context.Context, jobID string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `INSERT INTO provider_sync_runs(provider,job_id,status) VALUES('proxmox',$1,'running') RETURNING id::text`, jobID).Scan(&id)
	return id, err
}
func (r *Postgres) FinishRun(ctx context.Context, id, status string, resources int, runErr error) error {
	message := ""
	if runErr != nil {
		message = runErr.Error()
	}
	_, err := r.db.Exec(ctx, `UPDATE provider_sync_runs SET status=$2,resources_seen=$3,error=NULLIF($4,''),finished_at=now(),integration_id=(SELECT id FROM integrations WHERE provider='proxmox' ORDER BY updated_at DESC LIMIT 1) WHERE id=$1`, id, status, resources, message)
	return err
}

func (r *Postgres) Sync(ctx context.Context, snapshot adapter.Snapshot, clusterName string) (int, error) {
	clusterName = strings.TrimSpace(clusterName)
	if clusterName == "" {
		clusterName = "Proxmox"
	}
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var integrationID, clusterID string
	config, _ := json.Marshal(map[string]any{"cluster": clusterName, "credential_source": "environment"})
	integrationName := "Proxmox " + clusterName
	err = tx.QueryRow(ctx, `INSERT INTO integrations(name,provider,status,config,last_checked_at) VALUES($1,'proxmox','connected',$2,$3) ON CONFLICT(provider,name) DO UPDATE SET status='connected',config=EXCLUDED.config,last_checked_at=EXCLUDED.last_checked_at,updated_at=now() RETURNING id::text`, integrationName, config, snapshot.CapturedAt).Scan(&integrationID)
	if err != nil {
		return 0, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO clusters(integration_id,name,kind,status,metadata) VALUES($1,$2,'proxmox','online',$3) ON CONFLICT(integration_id,name) DO UPDATE SET status='online',metadata=EXCLUDED.metadata RETURNING id::text`, integrationID, clusterName, config).Scan(&clusterID)
	if err != nil {
		return 0, err
	}
	nodeIDs := map[string]string{}
	resources := 0
	for _, node := range snapshot.Nodes {
		facts, _ := json.Marshal(map[string]any{"max_cpu": node.MaxCPU, "max_memory": node.MaxMemory, "uptime": node.Uptime, "subscription_level": node.Level, "telemetry_source": "proxmox_rrd", "metrics": node.Metrics})
		var hostID, nodeID string
		hostName := "proxmox:" + clusterName + ":" + node.Node
		err = tx.QueryRow(ctx, `INSERT INTO hosts(integration_id,name,hostname,scope,status,self_protected,facts,last_seen_at) VALUES($1,$2,$3,'remote',$4,false,$5,$6) ON CONFLICT(name) DO UPDATE SET integration_id=EXCLUDED.integration_id,status=EXCLUDED.status,facts=EXCLUDED.facts,last_seen_at=EXCLUDED.last_seen_at RETURNING id::text`, integrationID, hostName, node.Node, resourceStatus(node.Status), facts, snapshot.CapturedAt).Scan(&hostID)
		if err != nil {
			return 0, err
		}
		metadata, _ := json.Marshal(map[string]any{"cpu_ratio": node.CPU, "memory_used": node.Memory, "uptime": node.Uptime, "metrics": node.Metrics})
		err = tx.QueryRow(ctx, `INSERT INTO nodes(cluster_id,host_id,external_id,name,status,cpu_cores,memory_bytes,metadata,last_seen_at) VALUES($1,$2,$3,$3,$4,$5,$6,$7,$8) ON CONFLICT(cluster_id,name) DO UPDATE SET host_id=EXCLUDED.host_id,status=EXCLUDED.status,cpu_cores=EXCLUDED.cpu_cores,memory_bytes=EXCLUDED.memory_bytes,metadata=EXCLUDED.metadata,last_seen_at=EXCLUDED.last_seen_at RETURNING id::text`, clusterID, hostID, node.Node, resourceStatus(node.Status), node.MaxCPU, node.MaxMemory, metadata, snapshot.CapturedAt).Scan(&nodeID)
		if err != nil {
			return 0, err
		}
		nodeIDs[node.Node] = nodeID
		resources++
		if err = insertMetric(ctx, tx, snapshot.CapturedAt, "node", nodeID, "cpu_usage_ratio", node.CPU, "ratio"); err != nil {
			return 0, err
		}
		if err = insertMetric(ctx, tx, snapshot.CapturedAt, "node", nodeID, "memory_used_bytes", float64(node.Memory), "bytes"); err != nil {
			return 0, err
		}
		// Mirror the Proxmox RRD point onto the managed host. This makes remote
		// hypervisors first-class telemetry targets without installing an agent
		// with command execution privileges on them.
		for _, metric := range []struct {
			name  string
			value float64
			unit  string
		}{
			{"proxmox_cpu_usage_ratio", node.Metrics.CPU, "ratio"},
			{"proxmox_load1", node.Metrics.Load1, "load"},
			{"proxmox_memory_total_bytes", float64(node.Metrics.MemoryTotal), "bytes"},
			{"proxmox_memory_available_bytes", float64(node.Metrics.MemoryAvailable), "bytes"},
			{"proxmox_root_total_bytes", float64(node.Metrics.RootTotal), "bytes"},
			{"proxmox_root_used_bytes", float64(node.Metrics.RootUsed), "bytes"},
			{"proxmox_network_receive_bytes_per_second", node.Metrics.NetworkInBPS, "bytes_per_second"},
			{"proxmox_network_transmit_bytes_per_second", node.Metrics.NetworkOutBPS, "bytes_per_second"},
			{"proxmox_io_wait_ratio", node.Metrics.IOWaitRatio, "ratio"},
		} {
			if err = insertMetric(ctx, tx, snapshot.CapturedAt, "host", hostID, metric.name, metric.value, metric.unit); err != nil {
				return 0, err
			}
		}
	}
	for _, vm := range snapshot.VMs {
		nodeID := nodeIDs[vm.Node]
		if nodeID == "" {
			return 0, fmt.Errorf("unknown Proxmox node %s", vm.Node)
		}
		name := vm.Name
		if name == "" {
			name = fmt.Sprintf("vm-%d", vm.VMID)
		}
		addresses := []byte("[]")
		metadata, _ := json.Marshal(map[string]any{"cpu_ratio": vm.CPU, "memory_used": vm.Memory, "uptime": vm.Uptime, "template": vm.Template})
		_, err = tx.Exec(ctx, `INSERT INTO virtual_machines(node_id,external_id,name,status,cpu_cores,memory_bytes,disk_bytes,addresses,metadata) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(node_id,external_id) DO UPDATE SET name=EXCLUDED.name,status=EXCLUDED.status,cpu_cores=EXCLUDED.cpu_cores,memory_bytes=EXCLUDED.memory_bytes,disk_bytes=EXCLUDED.disk_bytes,metadata=EXCLUDED.metadata`, nodeID, fmt.Sprint(vm.VMID), name, resourceStatus(vm.Status), vm.CPUs, vm.MaxMemory, vm.MaxDisk, addresses, metadata)
		if err != nil {
			return 0, err
		}
		resources++
	}
	for _, ct := range snapshot.Containers {
		hostID := ""
		if nodeID := nodeIDs[ct.Node]; nodeID != "" {
			_ = tx.QueryRow(ctx, `SELECT host_id::text FROM nodes WHERE id=$1`, nodeID).Scan(&hostID)
		}
		name := ct.Name
		if name == "" {
			name = fmt.Sprintf("ct-%d", ct.VMID)
		}
		metadata, _ := json.Marshal(map[string]any{"node": ct.Node, "cpu_ratio": ct.CPU, "memory_used": ct.Memory, "max_memory": ct.MaxMemory, "uptime": ct.Uptime})
		runtime := "proxmox-lxc:" + clusterName
		_, err = tx.Exec(ctx, `INSERT INTO containers(host_id,external_id,name,runtime,status,metadata) VALUES(NULLIF($1,'')::uuid,$2,$3,$4,$5,$6) ON CONFLICT(runtime,external_id) DO UPDATE SET host_id=EXCLUDED.host_id,name=EXCLUDED.name,status=EXCLUDED.status,metadata=EXCLUDED.metadata`, hostID, fmt.Sprint(ct.VMID), name, runtime, resourceStatus(ct.Status), metadata)
		if err != nil {
			return 0, err
		}
		resources++
	}
	for _, storage := range snapshot.Storage {
		nodeID := nodeIDs[storage.Node]
		if nodeID == "" {
			continue
		}
		storageState := "offline"
		if storage.Active == 1 {
			storageState = "online"
		}
		_, err = tx.Exec(ctx, `INSERT INTO infrastructure_storage_pools(integration_id,node_id,external_id,kind,status,total_bytes,used_bytes,available_bytes,shared,last_seen_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(integration_id,node_id,external_id) DO UPDATE SET kind=EXCLUDED.kind,status=EXCLUDED.status,total_bytes=EXCLUDED.total_bytes,used_bytes=EXCLUDED.used_bytes,available_bytes=EXCLUDED.available_bytes,shared=EXCLUDED.shared,last_seen_at=EXCLUDED.last_seen_at`, integrationID, nodeID, storage.Storage, storage.Type, storageState, storage.Total, storage.Used, storage.Available, storage.Shared == 1, snapshot.CapturedAt)
		if err != nil {
			return 0, err
		}
		resources++
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return resources, nil
}
func (r *Postgres) MarkError(ctx context.Context, message string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO integrations(name,provider,status,config,last_checked_at) VALUES('Proxmox','proxmox','error','{}',now()) ON CONFLICT(provider,name) DO UPDATE SET status='error',last_checked_at=now(),updated_at=now()`)
	return err
}
func (r *Postgres) Summary(ctx context.Context, configured bool) (Summary, error) {
	result := Summary{Configured: configured, Status: "unconfigured"}
	_ = r.db.QueryRow(ctx, `SELECT status::text,last_checked_at FROM integrations WHERE provider='proxmox' ORDER BY updated_at DESC LIMIT 1`).Scan(&result.Status, &result.LastCheckedAt)
	if !configured {
		return result, nil
	}
	_ = r.db.QueryRow(ctx, `SELECT count(*) FROM nodes n JOIN clusters c ON c.id=n.cluster_id WHERE c.kind='proxmox'`).Scan(&result.Nodes)
	_ = r.db.QueryRow(ctx, `SELECT count(*) FROM virtual_machines vm JOIN nodes n ON n.id=vm.node_id JOIN clusters c ON c.id=n.cluster_id WHERE c.kind='proxmox'`).Scan(&result.VMs)
	_ = r.db.QueryRow(ctx, `SELECT count(*) FROM containers WHERE runtime LIKE 'proxmox-lxc:%' OR runtime='proxmox-lxc'`).Scan(&result.Containers)
	_ = r.db.QueryRow(ctx, `SELECT count(*) FROM infrastructure_storage_pools`).Scan(&result.Storage)
	return result, nil
}
func resourceStatus(value string) string {
	switch value {
	case "online", "running":
		return "online"
	case "offline":
		return "offline"
	case "stopped":
		return "stopped"
	default:
		return "unknown"
	}
}
func insertMetric(ctx context.Context, tx pgx.Tx, at time.Time, resourceType, id, metric string, value float64, unit string) error {
	_, err := tx.Exec(ctx, `INSERT INTO metrics_snapshots(captured_at,resource_type,resource_id,metric,value,unit) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, at, resourceType, id, metric, value, unit)
	return err
}
