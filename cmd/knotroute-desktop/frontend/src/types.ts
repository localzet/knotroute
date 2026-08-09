export interface Snapshot {
  version: string; running: boolean; operation?: string; error?: string; integration: boolean; startup: boolean;
  overview: { state: 'online'|'starting'|'offline'; state_detail: string; peer_count: number; route_count: number; service_count: number; transport: string; node_domain: string; network_id: string; bytes_sent: number; bytes_recv: number };
  profile: { id: string; display_name: string; bio: string };
  contacts: Contact[]; posts: Post[]; catalog: Catalog[];
  network: { network_id: string; beacons: string[]; circuit_hops: number; transport_mode: string; direct_first: boolean; xray_socks: string; peers: Peer[]; routes: Route[]; transport_attempts: TransportAttempt[] };
  developer: { node_id: string; node_domain: string; listen: string[]; services: Service[]; known_services: KnownService[]; aliases: Alias[]; forwards: unknown[]; events: EventItem[]; active_streams: number; active_circuits: number; descriptors: number };
  ca: CA;
  paths: { data: string; config: string; identity: string; user_identity: string; social: string; ca: string };
}
export interface Contact { id:string; node:string; display_name:string; alias?:string; bio?:string; messages:Message[]; last_message?:string; last_unix?:number }
export interface Message { id:string; mine:boolean; body:string; unix:number; sender_id:string }
export interface Post { id:string; author_id:string; author:string; text:string; tags:string[]; unix:number; mine:boolean }
export interface Catalog { kind:string; title:string; description:string; domain:string; url:string; tags:string[]; hops?:number }
export interface Peer { id:string; short_id:string; direction:string; remote_addr:string; advertise:string[] }
export interface Route { destination:string; domain:string; short_id:string; next_hop:string; hops:number; path:string[]; services:string[] }
export interface TransportAttempt { name:string; type:string; target:string; duration:number; succeeded:boolean; error?:string; at:string }
export interface Service { name:string; target:string; description?:string; published:boolean; service_id?:string; domain?:string; introduction_points?:string[] }
export interface KnownService { service_id:string; domain:string; metadata:Record<string,string>; revision:number; expires_unix:number; introduction_points:string[] }
export interface Alias { name:string; kind:string; target:string; domain:string; description?:string }
export interface EventItem { time:string; level:string; message:string }
export interface CA { enabled:boolean; intercept_https:boolean; validity_days:number; common_name:string; organization:string[]; organizational_unit:string[]; country:string[]; province:string[]; locality:string[]; street_address:string[]; postal_code:string[]; fingerprint?:string; subject?:string; issuer?:string; not_after?:string }
export interface NetworkInput { network_id:string; beacons:string[]; circuit_hops:number; transport_mode:string; direct_first:boolean; xray_socks:string }
export interface CAInput { enabled:boolean; intercept_https:boolean; validity_days:number; common_name:string; organization:string[]; organizational_unit:string[]; country:string[]; province:string[]; locality:string[]; street_address:string[]; postal_code:string[] }
