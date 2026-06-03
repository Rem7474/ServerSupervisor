-- =====================================================================
-- V2 CONSOLIDATED BASELINE (squash of migrations 001..063)
-- =====================================================================
-- Generated from `pg_dump --schema-only` of a database migrated through 064 on
-- plain PostgreSQL (so disk_info / metrics_aggregates are already gone and no
-- TimescaleDB objects are present). Creates the full relational schema for
-- FRESH installs only.
--
-- TimescaleDB is NOT set up here: migration 064_v2_timescale_migrate.sql runs on
-- every install (it is not subsumed by this baseline) and is the single source
-- of truth for hypertable conversion / compression / retention. The continuous
-- aggregate is created from Go (DB.ensureTimescaleObjects).
--
-- Runner behaviour (see db.go): on an EXISTING database (table `hosts` present)
-- this baseline is recorded as applied WITHOUT executing; the INSERT at the end
-- marks every squashed migration (001..063) as applied so they never re-run.
-- =====================================================================

--
-- PostgreSQL database dump
--


-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14




--
-- Name: alert_incidents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.alert_incidents (
    id bigint NOT NULL,
    rule_id integer,
    host_id character varying(64),
    triggered_at timestamp with time zone DEFAULT now(),
    resolved_at timestamp with time zone,
    value double precision,
    severity character varying(10) DEFAULT 'crit'::character varying,
    CONSTRAINT alert_incidents_severity_check CHECK (((severity)::text = ANY ((ARRAY['warn'::character varying, 'crit'::character varying])::text[])))
);


--
-- Name: alert_incidents_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.alert_incidents_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: alert_incidents_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.alert_incidents_id_seq OWNED BY public.alert_incidents.id;


--
-- Name: alert_rules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.alert_rules (
    id integer NOT NULL,
    name character varying(255),
    source_type character varying(20) DEFAULT 'agent'::character varying NOT NULL,
    host_id character varying(64),
    proxmox_scope jsonb,
    metric character varying(50) NOT NULL,
    operator character varying(5) NOT NULL,
    threshold_crit double precision,
    duration_seconds integer DEFAULT 60,
    actions jsonb DEFAULT '{}'::jsonb NOT NULL,
    last_fired timestamp with time zone,
    enabled boolean DEFAULT true,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    threshold_clear_crit double precision,
    threshold_warn double precision,
    threshold_clear_warn double precision,
    CONSTRAINT chk_alert_rules_source_type CHECK (((source_type)::text = ANY ((ARRAY['agent'::character varying, 'proxmox'::character varying, 'synthetic'::character varying])::text[])))
);


--
-- Name: alert_rules_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.alert_rules_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: alert_rules_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.alert_rules_id_seq OWNED BY public.alert_rules.id;


--
-- Name: apt_status; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.apt_status (
    id bigint NOT NULL,
    host_id character varying(64),
    last_update timestamp with time zone,
    last_upgrade timestamp with time zone,
    pending_packages integer DEFAULT 0,
    package_list jsonb DEFAULT '[]'::jsonb,
    security_updates integer DEFAULT 0,
    cve_list jsonb DEFAULT '[]'::jsonb,
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: apt_status_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.apt_status_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: apt_status_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.apt_status_id_seq OWNED BY public.apt_status.id;


--
-- Name: audit_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.audit_logs (
    id bigint NOT NULL,
    username character varying(255) NOT NULL,
    action character varying(100) NOT NULL,
    host_id character varying(64),
    ip_address character varying(45),
    details text,
    status character varying(20) DEFAULT 'pending'::character varying,
    created_at timestamp with time zone DEFAULT now()
);


--
-- Name: audit_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.audit_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: audit_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.audit_logs_id_seq OWNED BY public.audit_logs.id;


--
-- Name: compose_projects; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.compose_projects (
    id character varying(255) NOT NULL,
    host_id character varying(64) NOT NULL,
    name character varying(255) NOT NULL,
    working_dir text DEFAULT ''::text NOT NULL,
    config_file text DEFAULT ''::text NOT NULL,
    services jsonb DEFAULT '[]'::jsonb NOT NULL,
    raw_config text DEFAULT ''::text NOT NULL,
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: disk_health; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.disk_health (
    id bigint NOT NULL,
    host_id character varying(64),
    "timestamp" timestamp with time zone DEFAULT now(),
    device character varying(255) NOT NULL,
    model character varying(255) DEFAULT ''::character varying NOT NULL,
    serial_number character varying(255) DEFAULT ''::character varying NOT NULL,
    smart_status character varying(50) DEFAULT 'UNKNOWN'::character varying NOT NULL,
    temperature integer DEFAULT 0,
    power_on_hours bigint DEFAULT 0,
    power_cycles bigint DEFAULT 0,
    realloc_sectors integer DEFAULT 0,
    pending_sectors integer DEFAULT 0
);


--
-- Name: disk_health_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.disk_health_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: disk_health_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.disk_health_id_seq OWNED BY public.disk_health.id;


--
-- Name: disk_metrics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.disk_metrics (
    id bigint NOT NULL,
    host_id character varying(64),
    "timestamp" timestamp with time zone DEFAULT now(),
    mount_point character varying(255) NOT NULL,
    filesystem character varying(255) DEFAULT ''::character varying NOT NULL,
    size_gb double precision DEFAULT 0,
    used_gb double precision DEFAULT 0,
    avail_gb double precision DEFAULT 0,
    used_percent double precision DEFAULT 0,
    inodes_total bigint DEFAULT 0,
    inodes_used bigint DEFAULT 0,
    inodes_free bigint DEFAULT 0,
    inodes_percent double precision DEFAULT 0
);


--
-- Name: disk_metrics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.disk_metrics_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: disk_metrics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.disk_metrics_id_seq OWNED BY public.disk_metrics.id;


--
-- Name: docker_containers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.docker_containers (
    id character varying(64) NOT NULL,
    host_id character varying(64),
    container_id character varying(64),
    name character varying(255),
    image character varying(512),
    image_tag character varying(255),
    image_id character varying(255),
    state character varying(50),
    status character varying(255),
    created timestamp with time zone,
    ports text,
    labels jsonb DEFAULT '{}'::jsonb,
    updated_at timestamp with time zone DEFAULT now(),
    env_vars jsonb DEFAULT '{}'::jsonb,
    volumes jsonb DEFAULT '[]'::jsonb,
    networks jsonb DEFAULT '[]'::jsonb,
    net_rx_bytes bigint DEFAULT 0,
    net_tx_bytes bigint DEFAULT 0,
    image_digest text DEFAULT ''::text NOT NULL
);


--
-- Name: docker_networks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.docker_networks (
    id character varying(64) NOT NULL,
    host_id character varying(64),
    network_id character varying(64) NOT NULL,
    name character varying(255) NOT NULL,
    driver character varying(50) DEFAULT 'bridge'::character varying,
    scope character varying(20) DEFAULT 'local'::character varying,
    container_ids jsonb DEFAULT '[]'::jsonb,
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: git_webhook_executions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.git_webhook_executions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    webhook_id uuid NOT NULL,
    command_id character varying(36),
    provider text DEFAULT ''::text NOT NULL,
    repo_name text DEFAULT ''::text NOT NULL,
    branch text DEFAULT ''::text NOT NULL,
    commit_sha text DEFAULT ''::text NOT NULL,
    commit_message text DEFAULT ''::text NOT NULL,
    pusher text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    triggered_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone
);


--
-- Name: git_webhooks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.git_webhooks (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    secret text NOT NULL,
    provider text DEFAULT 'github'::text NOT NULL,
    repo_filter text DEFAULT ''::text NOT NULL,
    branch_filter text DEFAULT ''::text NOT NULL,
    event_filter text DEFAULT 'push'::text NOT NULL,
    host_id character varying(64) NOT NULL,
    custom_task_id text NOT NULL,
    notify_channels text[] DEFAULT '{}'::text[] NOT NULL,
    notify_on_success boolean DEFAULT false NOT NULL,
    notify_on_failure boolean DEFAULT true NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    last_triggered_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: host_permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.host_permissions (
    username text NOT NULL,
    host_id text NOT NULL,
    level text DEFAULT 'viewer'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT host_permissions_level_check CHECK ((level = ANY (ARRAY['viewer'::text, 'operator'::text])))
);


--
-- Name: hosts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.hosts (
    id character varying(64) NOT NULL,
    name character varying(255) NOT NULL,
    hostname character varying(255) DEFAULT ''::character varying NOT NULL,
    ip_address character varying(45) NOT NULL,
    os character varying(255) DEFAULT ''::character varying NOT NULL,
    api_key character varying(255) NOT NULL,
    tags jsonb DEFAULT '[]'::jsonb,
    status character varying(20) DEFAULT 'offline'::character varying NOT NULL,
    last_seen timestamp with time zone DEFAULT now(),
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    agent_version character varying(20) DEFAULT ''::character varying,
    custom_tasks jsonb DEFAULT '[]'::jsonb NOT NULL,
    web_log_source text,
    web_log_collected_at timestamp with time zone,
    web_log_total_requests integer DEFAULT 0 NOT NULL,
    web_log_total_bytes bigint DEFAULT 0 NOT NULL,
    web_log_errors_4xx integer DEFAULT 0 NOT NULL,
    web_log_errors_5xx integer DEFAULT 0 NOT NULL,
    web_log_suspicious_requests integer DEFAULT 0 NOT NULL,
    web_log_suspicious_ips integer DEFAULT 0 NOT NULL,
    collectors jsonb DEFAULT '{"apt": false, "smart": false, "docker": false, "journal": false, "systemd": false, "cpu_temp": false, "web_logs": false}'::jsonb,
    tasks_config_yaml text DEFAULT ''::text NOT NULL
);


--
-- Name: ip_block_overrides; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ip_block_overrides (
    ip_address character varying(45) NOT NULL,
    unblocked_at timestamp with time zone DEFAULT now() NOT NULL,
    unblocked_by character varying(255) DEFAULT ''::character varying NOT NULL
);


--
-- Name: login_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.login_events (
    id bigint NOT NULL,
    username character varying(255) NOT NULL,
    ip_address character varying(45) DEFAULT ''::character varying NOT NULL,
    success boolean NOT NULL,
    user_agent character varying(500) DEFAULT ''::character varying,
    created_at timestamp with time zone DEFAULT now()
);


--
-- Name: login_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.login_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: login_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.login_events_id_seq OWNED BY public.login_events.id;


--
-- Name: network_topology_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.network_topology_config (
    id integer NOT NULL,
    root_label character varying(255) DEFAULT 'Infrastructure'::character varying,
    root_ip character varying(45) DEFAULT ''::character varying,
    excluded_ports jsonb DEFAULT '[]'::jsonb,
    service_map jsonb DEFAULT '{}'::jsonb,
    show_proxy_links boolean DEFAULT true,
    host_overrides jsonb DEFAULT '{}'::jsonb,
    manual_services jsonb DEFAULT '[]'::jsonb,
    updated_at timestamp with time zone DEFAULT now(),
    authelia_label character varying(255) DEFAULT 'Authelia'::character varying,
    authelia_ip character varying(45) DEFAULT ''::character varying,
    internet_label character varying(255) DEFAULT 'Internet'::character varying,
    internet_ip character varying(45) DEFAULT ''::character varying,
    node_positions jsonb DEFAULT '{}'::jsonb,
    root_host_id character varying(255) DEFAULT ''::character varying NOT NULL,
    authelia_host_id character varying(255) DEFAULT ''::character varying NOT NULL,
    root_port_id character varying(20) DEFAULT ''::character varying NOT NULL,
    authelia_port_id character varying(20) DEFAULT ''::character varying NOT NULL
);


--
-- Name: network_topology_config_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.network_topology_config_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: network_topology_config_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.network_topology_config_id_seq OWNED BY public.network_topology_config.id;


--
-- Name: notification_read_at; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notification_read_at (
    username text NOT NULL,
    read_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: proxmox_backup_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.proxmox_backup_jobs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    connection_id uuid NOT NULL,
    job_id text NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    schedule text DEFAULT ''::text NOT NULL,
    storage text DEFAULT ''::text NOT NULL,
    mode text DEFAULT 'snapshot'::text NOT NULL,
    compress text DEFAULT ''::text NOT NULL,
    vmids text DEFAULT ''::text NOT NULL,
    mail_to text DEFAULT ''::text NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: proxmox_backup_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.proxmox_backup_runs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    connection_id uuid NOT NULL,
    node_name text DEFAULT ''::text NOT NULL,
    vmid integer NOT NULL,
    task_upid text DEFAULT ''::text NOT NULL,
    status text DEFAULT ''::text NOT NULL,
    start_time timestamp with time zone,
    end_time timestamp with time zone,
    exit_status text DEFAULT ''::text NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: proxmox_connections; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.proxmox_connections (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    api_url text NOT NULL,
    token_id text NOT NULL,
    token_secret text NOT NULL,
    insecure_skip_verify boolean DEFAULT false NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    poll_interval_sec integer DEFAULT 60 NOT NULL,
    last_error text DEFAULT ''::text NOT NULL,
    last_error_at timestamp with time zone,
    last_success_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: proxmox_disks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.proxmox_disks (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    connection_id uuid NOT NULL,
    node_name text NOT NULL,
    dev_path text NOT NULL,
    model text DEFAULT ''::text NOT NULL,
    serial text DEFAULT ''::text NOT NULL,
    size_bytes bigint DEFAULT 0 NOT NULL,
    disk_type text DEFAULT ''::text NOT NULL,
    health text DEFAULT 'UNKNOWN'::text NOT NULL,
    wearout integer DEFAULT '-1'::integer NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: proxmox_guest_links; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.proxmox_guest_links (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    guest_id uuid NOT NULL,
    host_id text NOT NULL,
    status text DEFAULT 'suggested'::text NOT NULL,
    metrics_source text DEFAULT 'auto'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT proxmox_guest_links_metrics_source_check CHECK ((metrics_source = ANY (ARRAY['auto'::text, 'agent'::text, 'proxmox'::text]))),
    CONSTRAINT proxmox_guest_links_status_check CHECK ((status = ANY (ARRAY['suggested'::text, 'confirmed'::text, 'ignored'::text])))
);


--
-- Name: proxmox_guest_metrics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.proxmox_guest_metrics (
    id bigint NOT NULL,
    guest_id uuid NOT NULL,
    cpu_usage double precision DEFAULT 0 NOT NULL,
    mem_total bigint DEFAULT 0 NOT NULL,
    mem_used bigint DEFAULT 0 NOT NULL,
    "timestamp" timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: proxmox_guest_metrics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.proxmox_guest_metrics_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: proxmox_guest_metrics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.proxmox_guest_metrics_id_seq OWNED BY public.proxmox_guest_metrics.id;


--
-- Name: proxmox_guests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.proxmox_guests (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    connection_id uuid NOT NULL,
    node_name text NOT NULL,
    guest_type text NOT NULL,
    vmid integer NOT NULL,
    name text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'unknown'::text NOT NULL,
    cpu_alloc double precision DEFAULT 0 NOT NULL,
    cpu_usage double precision DEFAULT 0 NOT NULL,
    mem_alloc bigint DEFAULT 0 NOT NULL,
    mem_usage bigint DEFAULT 0 NOT NULL,
    disk_alloc bigint DEFAULT 0 NOT NULL,
    tags text DEFAULT ''::text NOT NULL,
    uptime bigint DEFAULT 0 NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: proxmox_node_metrics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.proxmox_node_metrics (
    id bigint NOT NULL,
    node_id uuid NOT NULL,
    connection_id uuid NOT NULL,
    node_name text NOT NULL,
    cpu_usage double precision DEFAULT 0 NOT NULL,
    mem_total bigint DEFAULT 0 NOT NULL,
    mem_used bigint DEFAULT 0 NOT NULL,
    "timestamp" timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: proxmox_node_metrics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.proxmox_node_metrics_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: proxmox_node_metrics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.proxmox_node_metrics_id_seq OWNED BY public.proxmox_node_metrics.id;


--
-- Name: proxmox_nodes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.proxmox_nodes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    connection_id uuid NOT NULL,
    node_name text NOT NULL,
    status text DEFAULT 'unknown'::text NOT NULL,
    cpu_count integer DEFAULT 0 NOT NULL,
    cpu_usage double precision DEFAULT 0 NOT NULL,
    mem_total bigint DEFAULT 0 NOT NULL,
    mem_used bigint DEFAULT 0 NOT NULL,
    uptime bigint DEFAULT 0 NOT NULL,
    pve_version text DEFAULT ''::text NOT NULL,
    cluster_name text DEFAULT ''::text NOT NULL,
    ip_address text DEFAULT ''::text NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL,
    pending_updates integer DEFAULT 0 NOT NULL,
    security_updates integer DEFAULT 0 NOT NULL,
    last_update_check_at timestamp with time zone,
    cpu_temp_source_host_id character varying(64),
    fan_rpm_source_host_id character varying(64)
);


--
-- Name: proxmox_storages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.proxmox_storages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    connection_id uuid NOT NULL,
    node_name text NOT NULL,
    storage_name text NOT NULL,
    storage_type text DEFAULT ''::text NOT NULL,
    total bigint DEFAULT 0 NOT NULL,
    used bigint DEFAULT 0 NOT NULL,
    avail bigint DEFAULT 0 NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    active boolean DEFAULT true NOT NULL,
    shared boolean DEFAULT false NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: proxmox_tasks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.proxmox_tasks (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    connection_id uuid NOT NULL,
    node_name text DEFAULT ''::text NOT NULL,
    upid text NOT NULL,
    task_type text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'stopped'::text NOT NULL,
    user_name text DEFAULT ''::text NOT NULL,
    start_time timestamp with time zone,
    end_time timestamp with time zone,
    exit_status text DEFAULT ''::text NOT NULL,
    object_id text DEFAULT ''::text NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: push_subscriptions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.push_subscriptions (
    id integer NOT NULL,
    username text NOT NULL,
    endpoint text NOT NULL,
    p256dh_key text NOT NULL,
    auth_key text NOT NULL,
    user_agent text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: push_subscriptions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.push_subscriptions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: push_subscriptions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.push_subscriptions_id_seq OWNED BY public.push_subscriptions.id;


--
-- Name: refresh_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.refresh_tokens (
    id bigint NOT NULL,
    user_id integer,
    token_hash character varying(64) NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now()
);


--
-- Name: refresh_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.refresh_tokens_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: refresh_tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.refresh_tokens_id_seq OWNED BY public.refresh_tokens.id;


--
-- Name: registry_credentials; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.registry_credentials (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(100) NOT NULL,
    registry_host character varying(255) NOT NULL,
    username character varying(255) NOT NULL,
    password text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: release_tracker_executions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.release_tracker_executions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tracker_id uuid NOT NULL,
    command_id character varying(36),
    tag_name text DEFAULT ''::text NOT NULL,
    release_url text DEFAULT ''::text NOT NULL,
    release_name text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    triggered_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone
);


--
-- Name: release_tracker_tag_digests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.release_tracker_tag_digests (
    tracker_id uuid NOT NULL,
    tag text NOT NULL,
    digest text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: release_trackers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.release_trackers (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    provider text DEFAULT 'github'::text NOT NULL,
    repo_owner text NOT NULL,
    repo_name text NOT NULL,
    host_id character varying(64),
    custom_task_id text DEFAULT ''::text NOT NULL,
    last_release_tag text DEFAULT ''::text NOT NULL,
    last_checked_at timestamp with time zone,
    last_triggered_at timestamp with time zone,
    notify_channels text[] DEFAULT '{}'::text[] NOT NULL,
    notify_on_release boolean DEFAULT true NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_error text DEFAULT ''::text NOT NULL,
    docker_image text DEFAULT ''::text NOT NULL,
    latest_image_digest text DEFAULT ''::text NOT NULL,
    tracker_type text DEFAULT 'git'::text NOT NULL,
    docker_tag text DEFAULT ''::text NOT NULL,
    cooldown_hours integer DEFAULT 0 NOT NULL,
    last_release_detected_at timestamp with time zone,
    update_action character varying(20) DEFAULT 'custom'::character varying NOT NULL,
    compose_project character varying(100),
    compose_service character varying(100),
    pre_update_task_id character varying(64),
    post_update_task_id character varying(64),
    cleanup_after_update boolean DEFAULT false NOT NULL,
    healthcheck_timeout_sec integer DEFAULT 0 NOT NULL,
    rollback_on_failure boolean DEFAULT false NOT NULL,
    registry_credentials_id uuid,
    CONSTRAINT release_trackers_compose_target_check CHECK ((((update_action)::text <> 'compose'::text) OR ((host_id IS NOT NULL) AND ((host_id)::text <> ''::text) AND (compose_project IS NOT NULL) AND ((compose_project)::text <> ''::text))))
);


--
-- Name: remote_commands; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.remote_commands (
    id character varying(36) NOT NULL,
    host_id character varying(64),
    module character varying(50) NOT NULL,
    action character varying(100) NOT NULL,
    target character varying(255) DEFAULT ''::character varying NOT NULL,
    payload text DEFAULT '{}'::text NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    output text DEFAULT ''::text NOT NULL,
    triggered_by character varying(255) DEFAULT 'system'::character varying NOT NULL,
    audit_log_id bigint,
    created_at timestamp with time zone DEFAULT now(),
    started_at timestamp with time zone,
    ended_at timestamp with time zone,
    scheduled_task_id uuid
);


--
-- Name: scheduled_tasks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scheduled_tasks (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    host_id character varying(64) NOT NULL,
    name text NOT NULL,
    module text NOT NULL,
    action text NOT NULL,
    target text DEFAULT ''::text NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    cron_expression text NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    last_run_at timestamp with time zone,
    next_run_at timestamp with time zone,
    last_run_status text,
    created_by text DEFAULT 'system'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--



--
-- Name: settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.settings (
    key character varying(100) NOT NULL,
    value text DEFAULT ''::text NOT NULL,
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: ssl_certificates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ssl_certificates (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    host text NOT NULL,
    port integer DEFAULT 443 NOT NULL,
    server_name text DEFAULT ''::text NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    last_checked_at timestamp with time zone,
    valid_from timestamp with time zone,
    valid_to timestamp with time zone,
    issuer text DEFAULT ''::text NOT NULL,
    subject text DEFAULT ''::text NOT NULL,
    serial_number text DEFAULT ''::text NOT NULL,
    dns_names text[] DEFAULT '{}'::text[] NOT NULL,
    last_error text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: system_metrics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.system_metrics (
    id bigint NOT NULL,
    host_id character varying(64),
    "timestamp" timestamp with time zone DEFAULT now(),
    cpu_usage_percent double precision,
    cpu_cores integer,
    cpu_model character varying(255),
    load_avg_1 double precision,
    load_avg_5 double precision,
    load_avg_15 double precision,
    memory_total bigint,
    memory_used bigint,
    memory_free bigint,
    memory_percent double precision,
    swap_total bigint,
    swap_used bigint,
    network_rx_bytes bigint,
    network_tx_bytes bigint,
    uptime bigint,
    hostname character varying(255),
    cpu_temperature double precision,
    fan_rpm double precision DEFAULT 0 NOT NULL
);


--
-- Name: system_metrics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.system_metrics_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: system_metrics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.system_metrics_id_seq OWNED BY public.system_metrics.id;


--
-- Name: unattended_upgrades_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.unattended_upgrades_runs (
    id bigint NOT NULL,
    host_id character varying(64) NOT NULL,
    run_at timestamp with time zone NOT NULL,
    packages jsonb DEFAULT '[]'::jsonb NOT NULL,
    had_error boolean DEFAULT false NOT NULL,
    log_snippet text,
    created_at timestamp with time zone DEFAULT now()
);


--
-- Name: unattended_upgrades_runs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.unattended_upgrades_runs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: unattended_upgrades_runs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.unattended_upgrades_runs_id_seq OWNED BY public.unattended_upgrades_runs.id;


--
-- Name: unattended_upgrades_status; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.unattended_upgrades_status (
    host_id character varying(64) NOT NULL,
    installed boolean DEFAULT false NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    reboot_required boolean DEFAULT false NOT NULL,
    last_run_at timestamp with time zone,
    last_run_packages integer DEFAULT 0,
    config jsonb DEFAULT '{}'::jsonb NOT NULL,
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: uptime_probe_results; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.uptime_probe_results (
    id bigint NOT NULL,
    probe_id uuid NOT NULL,
    checked_at timestamp with time zone DEFAULT now() NOT NULL,
    success boolean NOT NULL,
    status_code integer,
    latency_ms integer DEFAULT 0 NOT NULL,
    error text DEFAULT ''::text NOT NULL
);


--
-- Name: uptime_probe_results_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.uptime_probe_results_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: uptime_probe_results_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.uptime_probe_results_id_seq OWNED BY public.uptime_probe_results.id;


--
-- Name: uptime_probes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.uptime_probes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    type text NOT NULL,
    target text NOT NULL,
    interval_sec integer DEFAULT 60 NOT NULL,
    timeout_sec integer DEFAULT 10 NOT NULL,
    expected_status integer DEFAULT 200 NOT NULL,
    expected_body_regex text DEFAULT ''::text NOT NULL,
    follow_redirects boolean DEFAULT true NOT NULL,
    verify_tls boolean DEFAULT true NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    last_status text DEFAULT 'unknown'::text NOT NULL,
    last_latency_ms integer,
    last_status_code integer,
    last_error text DEFAULT ''::text NOT NULL,
    last_checked_at timestamp with time zone,
    consecutive_failures integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT uptime_probes_type_check CHECK ((type = ANY (ARRAY['http'::text, 'tcp'::text])))
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id integer NOT NULL,
    username character varying(255) NOT NULL,
    password_hash text NOT NULL,
    role character varying(50) DEFAULT 'viewer'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    totp_secret text DEFAULT ''::text,
    backup_codes jsonb DEFAULT '[]'::jsonb,
    mfa_enabled boolean DEFAULT false,
    must_change_password boolean DEFAULT false NOT NULL
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: web_log_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.web_log_requests (
    id bigint NOT NULL,
    snapshot_id bigint NOT NULL,
    host_id character varying(64) NOT NULL,
    captured_at timestamp with time zone NOT NULL,
    source text NOT NULL,
    ip text NOT NULL,
    method text NOT NULL,
    path text NOT NULL,
    status integer NOT NULL,
    bytes bigint DEFAULT 0 NOT NULL,
    user_agent text,
    domain text,
    category text,
    suspicious boolean DEFAULT false NOT NULL,
    fingerprint text NOT NULL,
    blocked boolean DEFAULT false NOT NULL,
    blocked_source text,
    blocked_reason text,
    blocked_at timestamp with time zone,
    blocked_until timestamp with time zone
);


--
-- Name: web_log_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.web_log_requests_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: web_log_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.web_log_requests_id_seq OWNED BY public.web_log_requests.id;


--
-- Name: web_log_snapshots; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.web_log_snapshots (
    id bigint NOT NULL,
    host_id character varying(64) NOT NULL,
    captured_at timestamp with time zone DEFAULT now() NOT NULL,
    source text NOT NULL,
    total_requests integer DEFAULT 0 NOT NULL,
    total_bytes bigint DEFAULT 0 NOT NULL,
    errors_4xx integer DEFAULT 0 NOT NULL,
    errors_5xx integer DEFAULT 0 NOT NULL,
    suspicious_requests integer DEFAULT 0 NOT NULL,
    suspicious_ips integer DEFAULT 0 NOT NULL,
    crowdsec_blocked_ips integer DEFAULT 0 NOT NULL,
    crowdsec_top_blocked jsonb DEFAULT '[]'::jsonb NOT NULL
);


--
-- Name: web_log_snapshots_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.web_log_snapshots_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: web_log_snapshots_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.web_log_snapshots_id_seq OWNED BY public.web_log_snapshots.id;


--
-- Name: alert_incidents id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alert_incidents ALTER COLUMN id SET DEFAULT nextval('public.alert_incidents_id_seq'::regclass);


--
-- Name: alert_rules id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alert_rules ALTER COLUMN id SET DEFAULT nextval('public.alert_rules_id_seq'::regclass);


--
-- Name: apt_status id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.apt_status ALTER COLUMN id SET DEFAULT nextval('public.apt_status_id_seq'::regclass);


--
-- Name: audit_logs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_logs ALTER COLUMN id SET DEFAULT nextval('public.audit_logs_id_seq'::regclass);


--
-- Name: disk_health id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disk_health ALTER COLUMN id SET DEFAULT nextval('public.disk_health_id_seq'::regclass);


--
-- Name: disk_metrics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disk_metrics ALTER COLUMN id SET DEFAULT nextval('public.disk_metrics_id_seq'::regclass);


--
-- Name: login_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.login_events ALTER COLUMN id SET DEFAULT nextval('public.login_events_id_seq'::regclass);


--
-- Name: network_topology_config id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.network_topology_config ALTER COLUMN id SET DEFAULT nextval('public.network_topology_config_id_seq'::regclass);


--
-- Name: proxmox_guest_metrics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_guest_metrics ALTER COLUMN id SET DEFAULT nextval('public.proxmox_guest_metrics_id_seq'::regclass);


--
-- Name: proxmox_node_metrics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_node_metrics ALTER COLUMN id SET DEFAULT nextval('public.proxmox_node_metrics_id_seq'::regclass);


--
-- Name: push_subscriptions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_subscriptions ALTER COLUMN id SET DEFAULT nextval('public.push_subscriptions_id_seq'::regclass);


--
-- Name: refresh_tokens id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens ALTER COLUMN id SET DEFAULT nextval('public.refresh_tokens_id_seq'::regclass);


--
-- Name: system_metrics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_metrics ALTER COLUMN id SET DEFAULT nextval('public.system_metrics_id_seq'::regclass);


--
-- Name: unattended_upgrades_runs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.unattended_upgrades_runs ALTER COLUMN id SET DEFAULT nextval('public.unattended_upgrades_runs_id_seq'::regclass);


--
-- Name: uptime_probe_results id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.uptime_probe_results ALTER COLUMN id SET DEFAULT nextval('public.uptime_probe_results_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Name: web_log_requests id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_log_requests ALTER COLUMN id SET DEFAULT nextval('public.web_log_requests_id_seq'::regclass);


--
-- Name: web_log_snapshots id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_log_snapshots ALTER COLUMN id SET DEFAULT nextval('public.web_log_snapshots_id_seq'::regclass);


--
-- Name: alert_incidents alert_incidents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alert_incidents
    ADD CONSTRAINT alert_incidents_pkey PRIMARY KEY (id);


--
-- Name: alert_rules alert_rules_rebuilt_pkey1; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alert_rules
    ADD CONSTRAINT alert_rules_rebuilt_pkey1 PRIMARY KEY (id);


--
-- Name: apt_status apt_status_host_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.apt_status
    ADD CONSTRAINT apt_status_host_id_key UNIQUE (host_id);


--
-- Name: apt_status apt_status_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.apt_status
    ADD CONSTRAINT apt_status_pkey PRIMARY KEY (id);


--
-- Name: audit_logs audit_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_logs
    ADD CONSTRAINT audit_logs_pkey PRIMARY KEY (id);


--
-- Name: compose_projects compose_projects_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.compose_projects
    ADD CONSTRAINT compose_projects_pkey PRIMARY KEY (id);


--
-- Name: disk_health disk_health_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disk_health
    ADD CONSTRAINT disk_health_pkey PRIMARY KEY (id);


--
-- Name: disk_metrics disk_metrics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disk_metrics
    ADD CONSTRAINT disk_metrics_pkey PRIMARY KEY (id);


--
-- Name: docker_containers docker_containers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.docker_containers
    ADD CONSTRAINT docker_containers_pkey PRIMARY KEY (id);


--
-- Name: docker_networks docker_networks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.docker_networks
    ADD CONSTRAINT docker_networks_pkey PRIMARY KEY (id);


--
-- Name: git_webhook_executions git_webhook_executions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.git_webhook_executions
    ADD CONSTRAINT git_webhook_executions_pkey PRIMARY KEY (id);


--
-- Name: git_webhooks git_webhooks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.git_webhooks
    ADD CONSTRAINT git_webhooks_pkey PRIMARY KEY (id);


--
-- Name: host_permissions host_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.host_permissions
    ADD CONSTRAINT host_permissions_pkey PRIMARY KEY (username, host_id);


--
-- Name: hosts hosts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.hosts
    ADD CONSTRAINT hosts_pkey PRIMARY KEY (id);


--
-- Name: ip_block_overrides ip_block_overrides_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ip_block_overrides
    ADD CONSTRAINT ip_block_overrides_pkey PRIMARY KEY (ip_address);


--
-- Name: login_events login_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.login_events
    ADD CONSTRAINT login_events_pkey PRIMARY KEY (id);


--
-- Name: network_topology_config network_topology_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.network_topology_config
    ADD CONSTRAINT network_topology_config_pkey PRIMARY KEY (id);


--
-- Name: notification_read_at notification_read_at_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_read_at
    ADD CONSTRAINT notification_read_at_pkey PRIMARY KEY (username);


--
-- Name: proxmox_backup_jobs proxmox_backup_jobs_connection_id_job_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_backup_jobs
    ADD CONSTRAINT proxmox_backup_jobs_connection_id_job_id_key UNIQUE (connection_id, job_id);


--
-- Name: proxmox_backup_jobs proxmox_backup_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_backup_jobs
    ADD CONSTRAINT proxmox_backup_jobs_pkey PRIMARY KEY (id);


--
-- Name: proxmox_backup_runs proxmox_backup_runs_connection_id_vmid_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_backup_runs
    ADD CONSTRAINT proxmox_backup_runs_connection_id_vmid_key UNIQUE (connection_id, vmid);


--
-- Name: proxmox_backup_runs proxmox_backup_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_backup_runs
    ADD CONSTRAINT proxmox_backup_runs_pkey PRIMARY KEY (id);


--
-- Name: proxmox_connections proxmox_connections_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_connections
    ADD CONSTRAINT proxmox_connections_pkey PRIMARY KEY (id);


--
-- Name: proxmox_disks proxmox_disks_connection_id_node_name_dev_path_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_disks
    ADD CONSTRAINT proxmox_disks_connection_id_node_name_dev_path_key UNIQUE (connection_id, node_name, dev_path);


--
-- Name: proxmox_disks proxmox_disks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_disks
    ADD CONSTRAINT proxmox_disks_pkey PRIMARY KEY (id);


--
-- Name: proxmox_guest_links proxmox_guest_links_guest_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_guest_links
    ADD CONSTRAINT proxmox_guest_links_guest_id_key UNIQUE (guest_id);


--
-- Name: proxmox_guest_links proxmox_guest_links_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_guest_links
    ADD CONSTRAINT proxmox_guest_links_pkey PRIMARY KEY (id);


--
-- Name: proxmox_guest_metrics proxmox_guest_metrics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_guest_metrics
    ADD CONSTRAINT proxmox_guest_metrics_pkey PRIMARY KEY (id);


--
-- Name: proxmox_guests proxmox_guests_connection_id_node_name_vmid_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_guests
    ADD CONSTRAINT proxmox_guests_connection_id_node_name_vmid_key UNIQUE (connection_id, node_name, vmid);


--
-- Name: proxmox_guests proxmox_guests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_guests
    ADD CONSTRAINT proxmox_guests_pkey PRIMARY KEY (id);


--
-- Name: proxmox_node_metrics proxmox_node_metrics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_node_metrics
    ADD CONSTRAINT proxmox_node_metrics_pkey PRIMARY KEY (id);


--
-- Name: proxmox_nodes proxmox_nodes_connection_id_node_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_nodes
    ADD CONSTRAINT proxmox_nodes_connection_id_node_name_key UNIQUE (connection_id, node_name);


--
-- Name: proxmox_nodes proxmox_nodes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_nodes
    ADD CONSTRAINT proxmox_nodes_pkey PRIMARY KEY (id);


--
-- Name: proxmox_storages proxmox_storages_connection_id_node_name_storage_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_storages
    ADD CONSTRAINT proxmox_storages_connection_id_node_name_storage_name_key UNIQUE (connection_id, node_name, storage_name);


--
-- Name: proxmox_storages proxmox_storages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_storages
    ADD CONSTRAINT proxmox_storages_pkey PRIMARY KEY (id);


--
-- Name: proxmox_tasks proxmox_tasks_connection_id_upid_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_tasks
    ADD CONSTRAINT proxmox_tasks_connection_id_upid_key UNIQUE (connection_id, upid);


--
-- Name: proxmox_tasks proxmox_tasks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_tasks
    ADD CONSTRAINT proxmox_tasks_pkey PRIMARY KEY (id);


--
-- Name: push_subscriptions push_subscriptions_endpoint_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_subscriptions
    ADD CONSTRAINT push_subscriptions_endpoint_key UNIQUE (endpoint);


--
-- Name: push_subscriptions push_subscriptions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_subscriptions
    ADD CONSTRAINT push_subscriptions_pkey PRIMARY KEY (id);


--
-- Name: refresh_tokens refresh_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_pkey PRIMARY KEY (id);


--
-- Name: refresh_tokens refresh_tokens_token_hash_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_token_hash_key UNIQUE (token_hash);


--
-- Name: registry_credentials registry_credentials_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.registry_credentials
    ADD CONSTRAINT registry_credentials_name_key UNIQUE (name);


--
-- Name: registry_credentials registry_credentials_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.registry_credentials
    ADD CONSTRAINT registry_credentials_pkey PRIMARY KEY (id);


--
-- Name: release_tracker_executions release_tracker_executions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.release_tracker_executions
    ADD CONSTRAINT release_tracker_executions_pkey PRIMARY KEY (id);


--
-- Name: release_tracker_tag_digests release_tracker_tag_digests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.release_tracker_tag_digests
    ADD CONSTRAINT release_tracker_tag_digests_pkey PRIMARY KEY (tracker_id, tag, digest);


--
-- Name: release_trackers release_trackers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.release_trackers
    ADD CONSTRAINT release_trackers_pkey PRIMARY KEY (id);


--
-- Name: remote_commands remote_commands_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.remote_commands
    ADD CONSTRAINT remote_commands_pkey PRIMARY KEY (id);


--
-- Name: scheduled_tasks scheduled_tasks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scheduled_tasks
    ADD CONSTRAINT scheduled_tasks_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--



--
-- Name: settings settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settings
    ADD CONSTRAINT settings_pkey PRIMARY KEY (key);


--
-- Name: ssl_certificates ssl_certificates_host_port_server_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ssl_certificates
    ADD CONSTRAINT ssl_certificates_host_port_server_name_key UNIQUE (host, port, server_name);


--
-- Name: ssl_certificates ssl_certificates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ssl_certificates
    ADD CONSTRAINT ssl_certificates_pkey PRIMARY KEY (id);


--
-- Name: system_metrics system_metrics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_metrics
    ADD CONSTRAINT system_metrics_pkey PRIMARY KEY (id);


--
-- Name: unattended_upgrades_runs unattended_upgrades_runs_host_id_run_at_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.unattended_upgrades_runs
    ADD CONSTRAINT unattended_upgrades_runs_host_id_run_at_key UNIQUE (host_id, run_at);


--
-- Name: unattended_upgrades_runs unattended_upgrades_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.unattended_upgrades_runs
    ADD CONSTRAINT unattended_upgrades_runs_pkey PRIMARY KEY (id);


--
-- Name: unattended_upgrades_status unattended_upgrades_status_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.unattended_upgrades_status
    ADD CONSTRAINT unattended_upgrades_status_pkey PRIMARY KEY (host_id);


--
-- Name: uptime_probe_results uptime_probe_results_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.uptime_probe_results
    ADD CONSTRAINT uptime_probe_results_pkey PRIMARY KEY (id);


--
-- Name: uptime_probes uptime_probes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.uptime_probes
    ADD CONSTRAINT uptime_probes_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_username_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_username_key UNIQUE (username);


--
-- Name: web_log_requests web_log_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_log_requests
    ADD CONSTRAINT web_log_requests_pkey PRIMARY KEY (id);


--
-- Name: web_log_snapshots web_log_snapshots_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_log_snapshots
    ADD CONSTRAINT web_log_snapshots_pkey PRIMARY KEY (id);


--
-- Name: idx_alert_incidents_rule; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_alert_incidents_rule ON public.alert_incidents USING btree (rule_id, triggered_at DESC);


--
-- Name: idx_alert_rules_host; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_alert_rules_host ON public.alert_rules USING btree (host_id);


--
-- Name: idx_alert_rules_source_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_alert_rules_source_type ON public.alert_rules USING btree (source_type);


--
-- Name: idx_audit_logs_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_logs_created ON public.audit_logs USING btree (created_at DESC);


--
-- Name: idx_audit_logs_host; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_logs_host ON public.audit_logs USING btree (host_id, created_at DESC);


--
-- Name: idx_audit_logs_user_action; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_logs_user_action ON public.audit_logs USING btree (username, action, created_at DESC);


--
-- Name: idx_audit_timestamp; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_timestamp ON public.audit_logs USING btree (created_at DESC);


--
-- Name: idx_commands_audit_log_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_commands_audit_log_id ON public.remote_commands USING btree (audit_log_id) WHERE (audit_log_id IS NOT NULL);


--
-- Name: idx_commands_host_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_commands_host_status ON public.remote_commands USING btree (host_id, status, created_at DESC);


--
-- Name: idx_compose_projects_host_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_compose_projects_host_id ON public.compose_projects USING btree (host_id);


--
-- Name: idx_disk_health_host_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_disk_health_host_time ON public.disk_health USING btree (host_id, "timestamp" DESC);


--
-- Name: idx_disk_metrics_host_mount; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_disk_metrics_host_mount ON public.disk_metrics USING btree (host_id, mount_point, "timestamp" DESC);


--
-- Name: idx_disk_metrics_host_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_disk_metrics_host_time ON public.disk_metrics USING btree (host_id, "timestamp" DESC);


--
-- Name: idx_docker_containers_host; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_docker_containers_host ON public.docker_containers USING btree (host_id);


--
-- Name: idx_docker_containers_image_digest; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_docker_containers_image_digest ON public.docker_containers USING btree (image_digest) WHERE (image_digest <> ''::text);


--
-- Name: idx_docker_containers_state; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_docker_containers_state ON public.docker_containers USING btree (state);


--
-- Name: idx_docker_networks_host; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_docker_networks_host ON public.docker_networks USING btree (host_id);


--
-- Name: idx_git_webhook_executions_command; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_git_webhook_executions_command ON public.git_webhook_executions USING btree (command_id);


--
-- Name: idx_git_webhook_executions_webhook; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_git_webhook_executions_webhook ON public.git_webhook_executions USING btree (webhook_id);


--
-- Name: idx_git_webhooks_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_git_webhooks_enabled ON public.git_webhooks USING btree (enabled) WHERE (enabled = true);


--
-- Name: idx_git_webhooks_host; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_git_webhooks_host ON public.git_webhooks USING btree (host_id);


--
-- Name: idx_host_permissions_host_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_host_permissions_host_id ON public.host_permissions USING btree (host_id);


--
-- Name: idx_host_permissions_username; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_host_permissions_username ON public.host_permissions USING btree (username);


--
-- Name: idx_hosts_collectors; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_hosts_collectors ON public.hosts USING gin (collectors);


--
-- Name: idx_login_events_ip_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_login_events_ip_time ON public.login_events USING btree (ip_address, created_at DESC);


--
-- Name: idx_login_events_user_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_login_events_user_time ON public.login_events USING btree (username, created_at DESC);


--
-- Name: idx_metrics_host_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_metrics_host_time ON public.system_metrics USING btree (host_id, "timestamp" DESC);


--
-- Name: idx_proxmox_backup_jobs_conn; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_backup_jobs_conn ON public.proxmox_backup_jobs USING btree (connection_id);


--
-- Name: idx_proxmox_backup_runs_conn; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_backup_runs_conn ON public.proxmox_backup_runs USING btree (connection_id);


--
-- Name: idx_proxmox_disks_node; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_disks_node ON public.proxmox_disks USING btree (connection_id, node_name);


--
-- Name: idx_proxmox_guest_links_host_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_guest_links_host_id ON public.proxmox_guest_links USING btree (host_id);


--
-- Name: idx_proxmox_guest_links_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_guest_links_status ON public.proxmox_guest_links USING btree (status);


--
-- Name: idx_proxmox_guest_metrics_guest_ts; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_guest_metrics_guest_ts ON public.proxmox_guest_metrics USING btree (guest_id, "timestamp" DESC);


--
-- Name: idx_proxmox_guests_conn; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_guests_conn ON public.proxmox_guests USING btree (connection_id);


--
-- Name: idx_proxmox_guests_node; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_guests_node ON public.proxmox_guests USING btree (connection_id, node_name);


--
-- Name: idx_proxmox_node_metrics_node_ts; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_node_metrics_node_ts ON public.proxmox_node_metrics USING btree (node_id, "timestamp" DESC);


--
-- Name: idx_proxmox_node_metrics_ts; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_node_metrics_ts ON public.proxmox_node_metrics USING btree ("timestamp" DESC);


--
-- Name: idx_proxmox_nodes_conn; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_nodes_conn ON public.proxmox_nodes USING btree (connection_id);


--
-- Name: idx_proxmox_nodes_cpu_temp_source_host; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_nodes_cpu_temp_source_host ON public.proxmox_nodes USING btree (cpu_temp_source_host_id);


--
-- Name: idx_proxmox_nodes_fan_rpm_source_host; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_nodes_fan_rpm_source_host ON public.proxmox_nodes USING btree (fan_rpm_source_host_id);


--
-- Name: idx_proxmox_storages_conn; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_storages_conn ON public.proxmox_storages USING btree (connection_id);


--
-- Name: idx_proxmox_storages_node; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_storages_node ON public.proxmox_storages USING btree (connection_id, node_name);


--
-- Name: idx_proxmox_tasks_conn_node; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_tasks_conn_node ON public.proxmox_tasks USING btree (connection_id, node_name);


--
-- Name: idx_proxmox_tasks_start_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_tasks_start_time ON public.proxmox_tasks USING btree (start_time DESC);


--
-- Name: idx_proxmox_tasks_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proxmox_tasks_type ON public.proxmox_tasks USING btree (task_type);


--
-- Name: idx_refresh_tokens_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_refresh_tokens_user ON public.refresh_tokens USING btree (user_id);


--
-- Name: idx_release_tracker_executions_command; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_release_tracker_executions_command ON public.release_tracker_executions USING btree (command_id);


--
-- Name: idx_release_tracker_executions_tracker; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_release_tracker_executions_tracker ON public.release_tracker_executions USING btree (tracker_id);


--
-- Name: idx_release_trackers_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_release_trackers_enabled ON public.release_trackers USING btree (enabled) WHERE (enabled = true);


--
-- Name: idx_release_trackers_host; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_release_trackers_host ON public.release_trackers USING btree (host_id);


--
-- Name: idx_remote_commands_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_remote_commands_created ON public.remote_commands USING btree (created_at DESC);


--
-- Name: idx_remote_commands_host_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_remote_commands_host_status ON public.remote_commands USING btree (host_id, status);


--
-- Name: idx_remote_commands_scheduled_task; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_remote_commands_scheduled_task ON public.remote_commands USING btree (scheduled_task_id) WHERE (scheduled_task_id IS NOT NULL);


--
-- Name: idx_remote_commands_triggered; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_remote_commands_triggered ON public.remote_commands USING btree (triggered_by);


--
-- Name: idx_rttd_tracker_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rttd_tracker_created ON public.release_tracker_tag_digests USING btree (tracker_id, created_at DESC);


--
-- Name: idx_rttd_tracker_digest; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rttd_tracker_digest ON public.release_tracker_tag_digests USING btree (tracker_id, digest);


--
-- Name: idx_scheduled_tasks_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_scheduled_tasks_enabled ON public.scheduled_tasks USING btree (enabled) WHERE (enabled = true);


--
-- Name: idx_scheduled_tasks_host; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_scheduled_tasks_host ON public.scheduled_tasks USING btree (host_id);


--
-- Name: idx_ssl_certificates_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ssl_certificates_enabled ON public.ssl_certificates USING btree (enabled) WHERE (enabled = true);


--
-- Name: idx_ssl_certificates_valid_to; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ssl_certificates_valid_to ON public.ssl_certificates USING btree (valid_to) WHERE (valid_to IS NOT NULL);


--
-- Name: idx_system_metrics_host_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_system_metrics_host_time ON public.system_metrics USING btree (host_id, "timestamp" DESC);


--
-- Name: idx_system_metrics_timestamp; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_system_metrics_timestamp ON public.system_metrics USING btree ("timestamp");


--
-- Name: idx_tracker_exec_tracker_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tracker_exec_tracker_time ON public.release_tracker_executions USING btree (tracker_id, triggered_at DESC);


--
-- Name: idx_uptime_probe_results_probe_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_uptime_probe_results_probe_time ON public.uptime_probe_results USING btree (probe_id, checked_at DESC);


--
-- Name: idx_uptime_probes_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_uptime_probes_enabled ON public.uptime_probes USING btree (enabled) WHERE (enabled = true);


--
-- Name: idx_uu_runs_host_run; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_uu_runs_host_run ON public.unattended_upgrades_runs USING btree (host_id, run_at DESC);


--
-- Name: idx_web_log_requests_blocked_captured; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_web_log_requests_blocked_captured ON public.web_log_requests USING btree (blocked, captured_at DESC);


--
-- Name: idx_web_log_requests_host_captured; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_web_log_requests_host_captured ON public.web_log_requests USING btree (host_id, captured_at DESC);


--
-- Name: idx_web_log_requests_ip_blocked; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_web_log_requests_ip_blocked ON public.web_log_requests USING btree (ip, blocked, captured_at DESC);


--
-- Name: idx_web_log_requests_ip_captured; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_web_log_requests_ip_captured ON public.web_log_requests USING btree (ip, captured_at DESC);


--
-- Name: idx_web_log_requests_source_captured; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_web_log_requests_source_captured ON public.web_log_requests USING btree (source, captured_at DESC);


--
-- Name: idx_web_log_requests_suspicious_captured; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_web_log_requests_suspicious_captured ON public.web_log_requests USING btree (suspicious, captured_at DESC);


--
-- Name: idx_web_log_snapshots_host_captured; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_web_log_snapshots_host_captured ON public.web_log_snapshots USING btree (host_id, captured_at DESC);


--
-- Name: idx_web_log_snapshots_host_source_captured; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_web_log_snapshots_host_source_captured ON public.web_log_snapshots USING btree (host_id, source, captured_at DESC);


--
-- Name: idx_webhook_exec_webhook_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_webhook_exec_webhook_time ON public.git_webhook_executions USING btree (webhook_id, triggered_at DESC);


--
-- Name: idx_wlr_host_blocked_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_wlr_host_blocked_at ON public.web_log_requests USING btree (host_id, blocked, captured_at DESC);


--
-- Name: idx_wlr_host_suspicious_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_wlr_host_suspicious_at ON public.web_log_requests USING btree (host_id, suspicious, captured_at DESC);


--
-- Name: network_topology_config_singleton; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX network_topology_config_singleton ON public.network_topology_config USING btree (id) WHERE (id = 1);


--
-- Name: ux_web_log_requests_host_source_fingerprint; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_web_log_requests_host_source_fingerprint ON public.web_log_requests USING btree (host_id, source, fingerprint);


--
-- Name: alert_incidents alert_incidents_rule_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alert_incidents
    ADD CONSTRAINT alert_incidents_rule_id_fkey FOREIGN KEY (rule_id) REFERENCES public.alert_rules(id) ON DELETE SET NULL;


--
-- Name: apt_status apt_status_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.apt_status
    ADD CONSTRAINT apt_status_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: compose_projects compose_projects_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.compose_projects
    ADD CONSTRAINT compose_projects_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: disk_health disk_health_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disk_health
    ADD CONSTRAINT disk_health_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: disk_metrics disk_metrics_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disk_metrics
    ADD CONSTRAINT disk_metrics_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: docker_containers docker_containers_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.docker_containers
    ADD CONSTRAINT docker_containers_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: docker_networks docker_networks_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.docker_networks
    ADD CONSTRAINT docker_networks_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: proxmox_nodes fk_proxmox_nodes_cpu_temp_source_host; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_nodes
    ADD CONSTRAINT fk_proxmox_nodes_cpu_temp_source_host FOREIGN KEY (cpu_temp_source_host_id) REFERENCES public.hosts(id) ON DELETE SET NULL;


--
-- Name: proxmox_nodes fk_proxmox_nodes_fan_rpm_source_host; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_nodes
    ADD CONSTRAINT fk_proxmox_nodes_fan_rpm_source_host FOREIGN KEY (fan_rpm_source_host_id) REFERENCES public.hosts(id) ON DELETE SET NULL;


--
-- Name: remote_commands fk_remote_commands_audit; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.remote_commands
    ADD CONSTRAINT fk_remote_commands_audit FOREIGN KEY (audit_log_id) REFERENCES public.audit_logs(id) ON DELETE SET NULL;


--
-- Name: git_webhook_executions git_webhook_executions_command_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.git_webhook_executions
    ADD CONSTRAINT git_webhook_executions_command_id_fkey FOREIGN KEY (command_id) REFERENCES public.remote_commands(id) ON DELETE SET NULL;


--
-- Name: git_webhook_executions git_webhook_executions_webhook_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.git_webhook_executions
    ADD CONSTRAINT git_webhook_executions_webhook_id_fkey FOREIGN KEY (webhook_id) REFERENCES public.git_webhooks(id) ON DELETE CASCADE;


--
-- Name: git_webhooks git_webhooks_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.git_webhooks
    ADD CONSTRAINT git_webhooks_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: host_permissions host_permissions_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.host_permissions
    ADD CONSTRAINT host_permissions_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: proxmox_backup_jobs proxmox_backup_jobs_connection_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_backup_jobs
    ADD CONSTRAINT proxmox_backup_jobs_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES public.proxmox_connections(id) ON DELETE CASCADE;


--
-- Name: proxmox_backup_runs proxmox_backup_runs_connection_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_backup_runs
    ADD CONSTRAINT proxmox_backup_runs_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES public.proxmox_connections(id) ON DELETE CASCADE;


--
-- Name: proxmox_disks proxmox_disks_connection_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_disks
    ADD CONSTRAINT proxmox_disks_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES public.proxmox_connections(id) ON DELETE CASCADE;


--
-- Name: proxmox_guest_links proxmox_guest_links_guest_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_guest_links
    ADD CONSTRAINT proxmox_guest_links_guest_id_fkey FOREIGN KEY (guest_id) REFERENCES public.proxmox_guests(id) ON DELETE CASCADE;


--
-- Name: proxmox_guest_links proxmox_guest_links_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_guest_links
    ADD CONSTRAINT proxmox_guest_links_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: proxmox_guest_metrics proxmox_guest_metrics_guest_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_guest_metrics
    ADD CONSTRAINT proxmox_guest_metrics_guest_id_fkey FOREIGN KEY (guest_id) REFERENCES public.proxmox_guests(id) ON DELETE CASCADE;


--
-- Name: proxmox_guests proxmox_guests_connection_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_guests
    ADD CONSTRAINT proxmox_guests_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES public.proxmox_connections(id) ON DELETE CASCADE;


--
-- Name: proxmox_node_metrics proxmox_node_metrics_node_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_node_metrics
    ADD CONSTRAINT proxmox_node_metrics_node_id_fkey FOREIGN KEY (node_id) REFERENCES public.proxmox_nodes(id) ON DELETE CASCADE;


--
-- Name: proxmox_nodes proxmox_nodes_connection_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_nodes
    ADD CONSTRAINT proxmox_nodes_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES public.proxmox_connections(id) ON DELETE CASCADE;


--
-- Name: proxmox_storages proxmox_storages_connection_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_storages
    ADD CONSTRAINT proxmox_storages_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES public.proxmox_connections(id) ON DELETE CASCADE;


--
-- Name: proxmox_tasks proxmox_tasks_connection_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proxmox_tasks
    ADD CONSTRAINT proxmox_tasks_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES public.proxmox_connections(id) ON DELETE CASCADE;


--
-- Name: refresh_tokens refresh_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: release_tracker_executions release_tracker_executions_command_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.release_tracker_executions
    ADD CONSTRAINT release_tracker_executions_command_id_fkey FOREIGN KEY (command_id) REFERENCES public.remote_commands(id) ON DELETE SET NULL;


--
-- Name: release_tracker_executions release_tracker_executions_tracker_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.release_tracker_executions
    ADD CONSTRAINT release_tracker_executions_tracker_id_fkey FOREIGN KEY (tracker_id) REFERENCES public.release_trackers(id) ON DELETE CASCADE;


--
-- Name: release_tracker_tag_digests release_tracker_tag_digests_tracker_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.release_tracker_tag_digests
    ADD CONSTRAINT release_tracker_tag_digests_tracker_id_fkey FOREIGN KEY (tracker_id) REFERENCES public.release_trackers(id) ON DELETE CASCADE;


--
-- Name: release_trackers release_trackers_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.release_trackers
    ADD CONSTRAINT release_trackers_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE SET NULL;


--
-- Name: release_trackers release_trackers_registry_credentials_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.release_trackers
    ADD CONSTRAINT release_trackers_registry_credentials_id_fkey FOREIGN KEY (registry_credentials_id) REFERENCES public.registry_credentials(id) ON DELETE SET NULL;


--
-- Name: remote_commands remote_commands_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.remote_commands
    ADD CONSTRAINT remote_commands_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: remote_commands remote_commands_scheduled_task_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.remote_commands
    ADD CONSTRAINT remote_commands_scheduled_task_id_fkey FOREIGN KEY (scheduled_task_id) REFERENCES public.scheduled_tasks(id) ON DELETE SET NULL;


--
-- Name: scheduled_tasks scheduled_tasks_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scheduled_tasks
    ADD CONSTRAINT scheduled_tasks_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: system_metrics system_metrics_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_metrics
    ADD CONSTRAINT system_metrics_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: unattended_upgrades_runs unattended_upgrades_runs_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.unattended_upgrades_runs
    ADD CONSTRAINT unattended_upgrades_runs_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: unattended_upgrades_status unattended_upgrades_status_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.unattended_upgrades_status
    ADD CONSTRAINT unattended_upgrades_status_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: uptime_probe_results uptime_probe_results_probe_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.uptime_probe_results
    ADD CONSTRAINT uptime_probe_results_probe_id_fkey FOREIGN KEY (probe_id) REFERENCES public.uptime_probes(id) ON DELETE CASCADE;


--
-- Name: web_log_requests web_log_requests_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_log_requests
    ADD CONSTRAINT web_log_requests_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: web_log_requests web_log_requests_snapshot_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_log_requests
    ADD CONSTRAINT web_log_requests_snapshot_id_fkey FOREIGN KEY (snapshot_id) REFERENCES public.web_log_snapshots(id) ON DELETE CASCADE;


--
-- Name: web_log_snapshots web_log_snapshots_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_log_snapshots
    ADD CONSTRAINT web_log_snapshots_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--



-- Mark all squashed migration files as applied so they are skipped after this baseline.
INSERT INTO schema_migrations (filename) VALUES
    ('001_core.sql'),
    ('002_aggregates.sql'),
    ('003_docker.sql'),
    ('004_topology.sql'),
    ('005_disk.sql'),
    ('006_settings.sql'),
    ('007_alter_columns.sql'),
    ('008_remote_commands.sql'),
    ('009_alert_actions.sql'),
    ('010_timescaledb.sql'),
    ('011_refactor.sql'),
    ('012_container_netstats.sql'),
    ('013_alert_incidents_fk.sql'),
    ('014_scheduled_tasks.sql'),
    ('015_scheduled_task_link.sql'),
    ('016_git_webhooks.sql'),
    ('017_release_trackers.sql'),
    ('018_release_tracker_error.sql'),
    ('019_release_tracker_docker_image.sql'),
    ('020_image_digest.sql'),
    ('021_performance_indexes.sql'),
    ('022_ip_block_overrides.sql'),
    ('023_tracker_tag_digests.sql'),
    ('024_topology_positions.sql'),
    ('025_push_notifications.sql'),
    ('026_memory_percent_aggregate.sql'),
    ('027_proxmox.sql'),
    ('028_proxmox_links.sql'),
    ('029_proxmox_extended.sql'),
    ('030_repair_alert_rules.sql'),
    ('031_tracker_type.sql'),
    ('032_proxmox_node_metrics.sql'),
    ('033_proxmox_guest_metrics.sql'),
    ('034_tracker_monitor_only.sql'),
    ('035_missing_indexes.sql'),
    ('036_host_permissions.sql'),
    ('037_bot_detection.sql'),
    ('038_npm_analytics.sql'),
    ('039_web_logs.sql'),
    ('040_web_logs_dedup_fingerprint.sql'),
    ('041_cpu_temperature.sql'),
    ('041_web_logs_crowdsec.sql'),
    ('042_proxmox_cpu_temp_source.sql'),
    ('043_host_collectors.sql'),
    ('044_alert_rules_legacy_cleanup.sql'),
    ('045_system_metrics_fan_rpm.sql'),
    ('046_proxmox_fan_rpm_source.sql'),
    ('047_alert_rule_source_type.sql'),
    ('048_alert_hysteresis.sql'),
    ('049_alert_severity_levels.sql'),
    ('050_tracker_history_cooldown.sql'),
    ('051_unattended_upgrades.sql'),
    ('052_crowdsec_blocked_ips.sql'),
    ('053_crowdsec_top_blocked.sql'),
    ('054_topology_host_links.sql'),
    ('055_topology_port_links.sql'),
    ('056_web_logs_indexes.sql'),
    ('057_tasks_config_yaml.sql'),
    ('058_uptime_probes.sql'),
    ('059_ssl_certificates.sql'),
    ('060_alert_rules_synthetic_source.sql'),
    ('061_drop_tracked_repos.sql'),
    ('062_compose_update_module.sql'),
    ('063_system_metrics_timestamp_index.sql');
