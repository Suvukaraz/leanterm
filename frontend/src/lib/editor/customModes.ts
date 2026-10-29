// Syntax modes for formats CodeMirror has no package for (CodeMirror 5 style
// stream parsers, wrapped with StreamLanguage in languages.ts).
import type { StringStream, StreamParser } from '@codemirror/language';

const words = (list: string) => new Set(list.split(/\s+/).filter(Boolean));

function hashComments(name: string, token: StreamParser<unknown>['token']): StreamParser<unknown> {
  return { name, token, languageData: { commentTokens: { line: '#' } } };
}

function readQuoted(stream: StringStream, quote: string): void {
  let escaped = false;
  let ch: string | void;
  while ((ch = stream.next()) != null) {
    if (ch === quote && !escaped) return;
    escaped = !escaped && ch === '\\';
  }
}

// ---- Nim ---------------------------------------------------------------------

const NIM_KEYWORDS = words(`addr and as asm bind block break case cast concept const continue converter
  defer discard distinct div do elif else end enum except export finally for from func if import in
  include interface is isnot iterator let macro method mixin mod nil not notin object of or out proc
  ptr raise ref return shl shr static template try tuple type using var when while xor yield`);
const NIM_TYPES = words(`int int8 int16 int32 int64 uint uint8 uint16 uint32 uint64 float float32 float64
  bool char string cstring pointer seq array set openArray varargs range auto void untyped typed
  Natural Positive byte`);
const NIM_ATOMS = words('true false nil');

interface NimState {
  comment: number;
  string: '' | '"""';
  defNext: boolean;
}

export const nim: StreamParser<NimState> = {
  name: 'nim',
  startState: () => ({ comment: 0, string: '', defNext: false }),
  token(stream, state) {
    if (state.comment > 0) {
      while (!stream.eol()) {
        if (stream.match('#[')) state.comment++;
        else if (stream.match(']#')) {
          state.comment--;
          if (state.comment === 0) break;
        } else stream.next();
      }
      return 'comment';
    }
    if (state.string) {
      if (stream.skipTo('"""')) {
        stream.match('"""');
        state.string = '';
      } else stream.skipToEnd();
      return 'string';
    }
    if (stream.eatSpace()) return null;
    if (stream.match('#[') || stream.match('##[')) {
      state.comment = 1;
      return 'comment';
    }
    if (stream.peek() === '#') {
      stream.skipToEnd();
      return 'comment';
    }
    if (stream.match(/^[rR]?"""/)) {
      state.string = '"""';
      return 'string';
    }
    if (stream.match(/^[rR]?"/)) {
      readQuoted(stream, '"');
      return 'string';
    }
    if (stream.match(/^'(\\.|[^'\\])'/)) return 'string';
    if (stream.match(/^\{\./)) {
      stream.skipTo('.}') ? stream.match('.}') : stream.skipToEnd();
      return 'meta';
    }
    if (stream.match(/^(0[xX][0-9a-fA-F_]+|0[oO][0-7_]+|0[bB][01_]+|\d[\d_]*(\.[\d_]+)?([eE][+-]?\d+)?)('?[iIuUfF]\d*)?/)) {
      return 'number';
    }
    if (stream.match(/^[A-Za-z_][\w]*/)) {
      const w = stream.current();
      if (state.defNext) {
        state.defNext = false;
        return 'def';
      }
      if (NIM_KEYWORDS.has(w)) {
        state.defNext = /^(proc|func|method|iterator|template|macro|converter|type)$/.test(w);
        return 'keyword';
      }
      if (NIM_ATOMS.has(w)) return 'atom';
      if (NIM_TYPES.has(w) || /^[A-Z]/.test(w)) return 'typeName';
      return 'variableName';
    }
    if (stream.match(/^[=+\-*/<>@$~&%|!?^.:\\]+/)) return 'operator';
    stream.next();
    return null;
  },
  languageData: { commentTokens: { line: '#', block: { open: '#[', close: ']#' } } },
};

// ---- Mojo (Python with Mojo's additions) ----------------------------------------

const MOJO_KEYWORDS = words(`and as assert async await break class continue def del elif else except finally
  for from global if import in is lambda match case nonlocal not or pass raise return try while with
  yield fn struct trait var let alias inout owned borrowed mut ref out raises capturing escaping
  comptime self Self`);
const MOJO_BUILTINS = words(`print len range int str float bool list dict set tuple abs min max sum any all
  Int Int8 Int16 Int32 Int64 UInt UInt8 UInt16 UInt32 UInt64 Float16 Float32 Float64 Bool String
  StringLiteral SIMD DType List Dict Optional Pointer UnsafePointer object AnyType`);

interface PyState {
  string: string;
  defNext: boolean;
}

export const mojo: StreamParser<PyState> = {
  name: 'mojo',
  startState: () => ({ string: '', defNext: false }),
  token(stream, state) {
    if (state.string) {
      if (stream.skipTo(state.string)) {
        stream.match(state.string);
        state.string = '';
      } else stream.skipToEnd();
      return 'string';
    }
    if (stream.eatSpace()) return null;
    if (stream.peek() === '#') {
      stream.skipToEnd();
      return 'comment';
    }
    const triple = stream.match(/^[rRbBfFuU]{0,2}("""|''')/) as RegExpMatchArray | null;
    if (triple) {
      state.string = triple[1];
      return 'string';
    }
    const single = stream.match(/^[rRbBfFuU]{0,2}(["'])/) as RegExpMatchArray | null;
    if (single) {
      readQuoted(stream, single[1]);
      return 'string';
    }
    if (stream.match(/^@[\w.]+/)) return 'meta';
    if (stream.match(/^(0[xX][0-9a-fA-F_]+|0[oO][0-7_]+|0[bB][01_]+|\d[\d_]*(\.[\d_]*)?([eE][+-]?\d+)?j?)/)) {
      return 'number';
    }
    if (stream.match(/^[A-Za-z_]\w*/)) {
      const w = stream.current();
      if (state.defNext) {
        state.defNext = false;
        return 'def';
      }
      if (w === 'True' || w === 'False' || w === 'None') return 'atom';
      if (MOJO_KEYWORDS.has(w)) {
        state.defNext = /^(def|fn|class|struct|trait|alias)$/.test(w);
        return 'keyword';
      }
      if (MOJO_BUILTINS.has(w)) return 'builtin';
      return 'variableName';
    }
    if (stream.match(/^[=+\-*/<>!&|^~%:.@-]+/)) return 'operator';
    stream.next();
    return null;
  },
  languageData: { commentTokens: { line: '#' } },
};

// ---- Makefile ------------------------------------------------------------------

const MAKE_DIRECTIVES = words(`include -include sinclude ifeq ifneq ifdef ifndef else endif define endef
  export unexport override undefine vpath private`);

function makeVariable(stream: StringStream): boolean {
  if (stream.match(/^\$[@<^+?*%|]/) || stream.match(/^\$\$/)) return true;
  const open = stream.match(/^\$([({])/) as RegExpMatchArray | null;
  if (!open) return false;
  const close = open[1] === '(' ? ')' : '}';
  let depth = 1;
  let ch: string | void;
  while ((ch = stream.next()) != null) {
    if (ch === open[1]) depth++;
    else if (ch === close && --depth === 0) break;
  }
  return true;
}

export const makefile: StreamParser<unknown> = hashComments('makefile', (stream) => {
  if (stream.sol() && stream.peek() === '\t') {
    // Recipe line (shell commands): only variables and comments stand out.
    stream.next();
  }
  if (stream.eatSpace()) return null;
  if (stream.peek() === '#') {
    stream.skipToEnd();
    return 'comment';
  }
  if (makeVariable(stream)) return 'variableName';
  if (stream.sol() || stream.string.slice(0, stream.pos).trim() === '') {
    if (stream.match(/^[\w.\-/%$(){}]+(?=\s*(::?=|\?=|\+=|!=|=))/)) return 'def';
    if (stream.match(/^[^\s:=#]+(?=\s*::?(?!=))/)) return 'def';
  }
  if (stream.match(/^[A-Za-z_-]+/)) {
    return MAKE_DIRECTIVES.has(stream.current()) ? 'keyword' : null;
  }
  if (stream.match(/^["']/)) {
    readQuoted(stream, stream.current());
    return 'string';
  }
  if (stream.match(/^(::?=|\?=|\+=|!=|=|::?|\|)/)) return 'operator';
  stream.next();
  return null;
});

// ---- Windows batch (.bat/.cmd) -----------------------------------------------------

const BATCH_KEYWORDS = words(`if else not exist defined errorlevel equ neq lss leq gtr geq for in do goto
  call set setlocal endlocal enabledelayedexpansion disabledelayedexpansion enableextensions shift exit
  echo pause start cd chdir pushd popd md mkdir rd rmdir del erase copy xcopy robocopy move ren rename
  type cls title color choice timeout where findstr find dir path prompt ver vol break nul con`);

export const batch: StreamParser<unknown> = {
  name: 'batch',
  token(stream) {
    if (stream.sol()) {
      stream.eatSpace();
      if (stream.match(/^(rem(\s|$)|::)/i)) {
        stream.skipToEnd();
        return 'comment';
      }
      if (stream.match(/^@/)) return 'operator';
      if (stream.match(/^:[^\s:]\S*/)) return 'def';
    }
    if (stream.eatSpace()) return null;
    if (stream.match(/^%%~?[a-zA-Z]/) || stream.match(/^%~?[0-9*]/) || stream.match(/^%[^%\s]+%/) || stream.match(/^![^!\s]+!/)) {
      return 'variableName';
    }
    if (stream.peek() === '"') {
      stream.next();
      readQuoted(stream, '"');
      return 'string';
    }
    if (stream.match(/^\d+\b/)) return 'number';
    if (stream.match(/^[A-Za-z_][\w.-]*/)) {
      return BATCH_KEYWORDS.has(stream.current().toLowerCase()) ? 'keyword' : null;
    }
    if (stream.match(/^(==|[<>|&]+|\^)/)) return 'operator';
    stream.next();
    return null;
  },
  languageData: { commentTokens: { line: 'REM ' } },
};

// ---- Windows registry (.reg) -----------------------------------------------------------

export const winRegistry: StreamParser<unknown> = {
  name: 'reg',
  token(stream) {
    if (stream.sol()) {
      if (stream.match(/^(Windows Registry Editor Version [\d.]+|REGEDIT4)\s*$/)) return 'meta';
      if (stream.match(/^\[-?[^\]]*\]/)) return 'heading';
    }
    if (stream.eatSpace()) return null;
    if (stream.peek() === ';') {
      stream.skipToEnd();
      return 'comment';
    }
    if (stream.peek() === '"') {
      stream.next();
      readQuoted(stream, '"');
      return stream.match(/^\s*=/, false) ? 'propertyName' : 'string';
    }
    if (stream.match(/^@(?=\s*=)/)) return 'propertyName';
    if (stream.match(/^(dword|qword|hex(\([0-9a-fA-F]+\))?):/i)) return 'typeName';
    if (stream.match(/^[0-9a-fA-F]+/)) return 'number';
    if (stream.match(/^[=,\\-]/)) return 'operator';
    stream.next();
    return null;
  },
  languageData: { commentTokens: { line: ';' } },
};

// ---- ssh_config / sshd_config -------------------------------------------------------------

export const sshConfig: StreamParser<{ blockLine: boolean }> = {
  name: 'sshconfig',
  startState: () => ({ blockLine: false }),
  token(stream, state) {
    if (stream.sol()) {
      state.blockLine = false;
      stream.eatSpace();
      if (stream.peek() === '#') {
        stream.skipToEnd();
        return 'comment';
      }
      if (stream.match(/^[A-Za-z][A-Za-z0-9]*/)) {
        state.blockLine = /^(host|match)$/i.test(stream.current());
        return state.blockLine ? 'heading' : 'keyword';
      }
    }
    if (stream.eatSpace()) return null;
    if (stream.peek() === '#') {
      stream.skipToEnd();
      return 'comment';
    }
    if (stream.peek() === '"') {
      stream.next();
      readQuoted(stream, '"');
      return 'string';
    }
    if (state.blockLine && stream.match(/^\S+/)) return 'def';
    if (stream.match(/^(yes|no|any|none|auto|ask)\b/i)) return 'atom';
    if (stream.match(/^\d+[smhdwKMG]?\b/)) return 'number';
    if (stream.match(/^%[a-zA-Z%]/) || stream.match(/^\$\{[^}]*\}/)) return 'variableName';
    if (stream.match(/^=/)) return 'operator';
    stream.next();
    return null;
  },
  languageData: { commentTokens: { line: '#' } },
};

// ---- crontab ---------------------------------------------------------------------------

export const crontab: StreamParser<{ field: number }> = {
  name: 'crontab',
  startState: () => ({ field: 0 }),
  token(stream, state) {
    if (stream.sol()) {
      state.field = 0;
      stream.eatSpace();
      if (stream.peek() === '#') {
        stream.skipToEnd();
        return 'comment';
      }
      if (stream.match(/^[A-Za-z_]\w*(?=\s*=)/)) {
        state.field = -1;
        return 'propertyName';
      }
      if (stream.match(/^@(reboot|yearly|annually|monthly|weekly|daily|midnight|hourly)\b/)) {
        state.field = 5;
        return 'keyword';
      }
    }
    if (stream.eatSpace()) return null;
    if (state.field >= 0 && state.field < 5 && stream.match(/^\S+/)) {
      state.field++;
      return 'number';
    }
    if (stream.match(/^=/)) return 'operator';
    if (stream.peek() === '"' || stream.peek() === "'") {
      const q = stream.next() as string;
      readQuoted(stream, q);
      return 'string';
    }
    if (stream.match(/^\$\{?\w+\}?/)) return 'variableName';
    stream.next();
    return null;
  },
  languageData: { commentTokens: { line: '#' } },
};

// ---- Apache httpd.conf / .htaccess ------------------------------------------------------

export const apache: StreamParser<unknown> = hashComments('apache', (stream) => {
  if (stream.sol()) stream.eatSpace();
  if (stream.peek() === '#' && stream.string.slice(0, stream.pos).trim() === '') {
    stream.skipToEnd();
    return 'comment';
  }
  if (stream.match(/^<\/?[A-Za-z]+/)) return 'tagName';
  if (stream.match(/^>/)) return 'tagName';
  if (stream.eatSpace()) return null;
  if (stream.string.slice(0, stream.pos).trim() === '' && stream.match(/^[A-Za-z][\w]*/)) return 'keyword';
  if (stream.peek() === '"' || stream.peek() === "'") {
    const q = stream.next() as string;
    readQuoted(stream, q);
    return 'string';
  }
  if (stream.match(/^[%$]\{[^}]*\}/)) return 'variableName';
  if (stream.match(/^\b(on|off|all|none)\b/i)) return 'atom';
  if (stream.match(/^\d+\b/)) return 'number';
  stream.next();
  return null;
});

// ---- .gitignore and friends --------------------------------------------------------------

export const ignoreFile: StreamParser<unknown> = hashComments('ignore', (stream) => {
  if (stream.sol() && stream.peek() === '#') {
    stream.skipToEnd();
    return 'comment';
  }
  if (stream.sol() && stream.match(/^!/)) return 'operator';
  if (stream.match(/^(\*\*|\*|\?|\[[^\]]*\])/)) return 'operator';
  if (stream.match(/^\//)) return 'punctuation';
  stream.next();
  return null;
});

// ---- Generic whitespace-separated config (hosts, fstab, resolv.conf, ...) ----------

export const genericConf: StreamParser<{ first: boolean }> = {
  name: 'conf',
  startState: () => ({ first: true }),
  token(stream, state) {
    if (stream.sol()) state.first = true;
    if (stream.eatSpace()) return null;
    if (stream.peek() === '#' || stream.peek() === ';') {
      stream.skipToEnd();
      return 'comment';
    }
    if (stream.peek() === '"' || stream.peek() === "'") {
      const q = stream.next() as string;
      readQuoted(stream, q);
      state.first = false;
      return 'string';
    }
    if (stream.match(/^(\d{1,3}\.){3}\d{1,3}(\/\d+)?/) || stream.match(/^[0-9a-fA-F]*:[0-9a-fA-F:]+(\/\d+)?/)) {
      state.first = false;
      return 'number';
    }
    if (stream.match(/^\d+(\.\d+)?\b/)) {
      state.first = false;
      return 'number';
    }
    if (stream.match(/^[^\s#;"'=]+/)) {
      const style = state.first ? 'keyword' : null;
      state.first = false;
      return style;
    }
    stream.next();
    return 'operator';
  },
  languageData: { commentTokens: { line: '#' } },
};

// ---- systemd units (.service, .socket, .timer, ...; also networkd, nspawn, quadlet) -------

const SYSTEMD_CONDITIONS = `Architecture Firmware Virtualization Host KernelCommandLine KernelVersion Credential
  Environment Security Capability ACPower NeedsUpdate FirstBoot PathExists PathExistsGlob PathIsDirectory
  PathIsSymbolicLink PathIsMountPoint PathIsReadWrite PathIsEncrypted DirectoryNotEmpty FileNotEmpty
  FileIsExecutable User Group ControlGroupController Memory CPUs CPUFeature OSRelease MemoryPressure
  CPUPressure IOPressure`
  .split(/\s+/)
  .filter(Boolean)
  .flatMap((c) => [`Condition${c}`, `Assert${c}`])
  .join(' ');

const SYSTEMD_KEYS = words(`${SYSTEMD_CONDITIONS}
  Description Documentation Wants Requires Requisite BindsTo PartOf Upholds Conflicts Before After OnFailure
  OnSuccess PropagatesReloadTo ReloadPropagatedFrom PropagatesStopTo StopPropagatedFrom JoinsNamespaceOf
  RequiresMountsFor WantsMountsFor OnFailureJobMode OnFailureIsolate IgnoreOnIsolate StopWhenUnneeded
  RefuseManualStart RefuseManualStop AllowIsolate DefaultDependencies SurviveFinalKillSignal CollectMode
  FailureAction SuccessAction FailureActionExitStatus SuccessActionExitStatus JobTimeoutSec
  JobRunningTimeoutSec JobTimeoutAction JobTimeoutRebootArgument StartLimitIntervalSec StartLimitBurst
  StartLimitAction RebootArgument SourcePath
  Alias WantedBy RequiredBy UpheldBy Also DefaultInstance
  Type ExitType RemainAfterExit GuessMainPID PIDFile BusName ExecStart ExecStartPre ExecStartPost ExecCondition
  ExecReload ExecStop ExecStopPre ExecStopPost RestartSec RestartSteps RestartMaxDelaySec TimeoutStartSec
  TimeoutStopSec TimeoutAbortSec TimeoutSec TimeoutStartFailureMode TimeoutStopFailureMode RuntimeMaxSec
  RuntimeRandomizedExtraSec WatchdogSec Restart RestartMode SuccessExitStatus RestartPreventExitStatus
  RestartForceExitStatus RootDirectoryStartOnly PermissionsStartOnly NonBlocking NotifyAccess Sockets
  FileDescriptorStoreMax FileDescriptorStorePreserve USBFunctionDescriptors USBFunctionStrings OOMPolicy
  OpenFile ReloadSignal
  ExecSearchPath WorkingDirectory RootDirectory RootImage RootImageOptions RootEphemeral RootHash
  RootHashSignature RootVerity RootImagePolicy MountImagePolicy ExtensionImagePolicy MountAPIVFS ProtectProc
  ProcSubset BindPaths BindReadOnlyPaths MountImages ExtensionImages ExtensionDirectories User Group
  DynamicUser SupplementaryGroups SetLoginEnvironment PAMName CapabilityBoundingSet AmbientCapabilities
  NoNewPrivileges SecureBits SELinuxContext AppArmorProfile SmackProcessLabel LimitCPU LimitFSIZE LimitDATA
  LimitSTACK LimitCORE LimitRSS LimitNOFILE LimitAS LimitNPROC LimitMEMLOCK LimitLOCKS LimitSIGPENDING
  LimitMSGQUEUE LimitNICE LimitRTPRIO LimitRTTIME UMask CoredumpFilter KeyringMode OOMScoreAdjust
  TimerSlackNSec Personality IgnoreSIGPIPE Nice CPUSchedulingPolicy CPUSchedulingPriority
  CPUSchedulingResetOnFork CPUAffinity NUMAPolicy NUMAMask IOSchedulingClass IOSchedulingPriority
  ProtectSystem ProtectHome RuntimeDirectory StateDirectory CacheDirectory LogsDirectory
  ConfigurationDirectory RuntimeDirectoryMode StateDirectoryMode CacheDirectoryMode LogsDirectoryMode
  ConfigurationDirectoryMode RuntimeDirectoryPreserve TimeoutCleanSec ReadWritePaths ReadOnlyPaths
  InaccessiblePaths ExecPaths NoExecPaths TemporaryFileSystem PrivateTmp PrivateDevices PrivateNetwork
  NetworkNamespacePath PrivateIPC IPCNamespacePath PrivatePIDs MemoryKSM PrivateUsers ProtectHostname
  ProtectClock ProtectKernelTunables ProtectKernelModules ProtectKernelLogs ProtectControlGroups
  RestrictAddressFamilies RestrictFileSystems RestrictNamespaces DelegateNamespaces LockPersonality
  MemoryDenyWriteExecute RestrictRealtime RestrictSUIDSGID RemoveIPC PrivateMounts MountFlags
  SystemCallFilter SystemCallErrorNumber SystemCallArchitectures SystemCallLog Environment EnvironmentFile
  PassEnvironment UnsetEnvironment StandardInput StandardOutput StandardError StandardInputText
  StandardInputData LogLevelMax LogExtraFields LogRateLimitIntervalSec LogRateLimitBurst LogFilterPatterns
  LogNamespace SyslogIdentifier SyslogFacility SyslogLevel SyslogLevelPrefix TTYPath TTYReset TTYVHangup
  TTYRows TTYColumns TTYVTDisallocate LoadCredential LoadCredentialEncrypted ImportCredential SetCredential
  SetCredentialEncrypted UtmpIdentifier UtmpMode
  KillMode KillSignal RestartKillSignal SendSIGHUP SendSIGKILL FinalKillSignal WatchdogSignal
  CPUAccounting CPUWeight StartupCPUWeight CPUQuota CPUQuotaPeriodSec AllowedCPUs StartupAllowedCPUs
  AllowedMemoryNodes StartupAllowedMemoryNodes MemoryAccounting MemoryMin MemoryLow StartupMemoryLow
  DefaultStartupMemoryLow DefaultMemoryLow DefaultMemoryMin MemoryHigh StartupMemoryHigh MemoryMax
  StartupMemoryMax MemorySwapMax StartupMemorySwapMax MemoryZSwapMax StartupMemoryZSwapMax
  MemoryZSwapWriteback TasksAccounting TasksMax IOAccounting IOWeight StartupIOWeight IODeviceWeight
  IOReadBandwidthMax IOWriteBandwidthMax IOReadIOPSMax IOWriteIOPSMax IODeviceLatencyTargetSec IPAccounting
  IPAddressAllow IPAddressDeny SocketBindAllow SocketBindDeny RestrictNetworkInterfaces NFTSet
  IPIngressFilterPath IPEgressFilterPath BPFProgram DeviceAllow DevicePolicy Slice Delegate DelegateSubgroup
  DisableControllers ManagedOOMSwap ManagedOOMMemoryPressure ManagedOOMMemoryPressureLimit
  ManagedOOMMemoryPressureDurationSec ManagedOOMPreference MemoryPressureWatch MemoryPressureThresholdSec
  CoredumpReceive CPUShares StartupCPUShares MemoryLimit BlockIOAccounting BlockIOWeight
  ListenStream ListenDatagram ListenSequentialPacket ListenFIFO ListenSpecial ListenNetlink ListenMessageQueue
  ListenUSBFunction SocketProtocol BindIPv6Only Backlog BindToDevice SocketUser SocketGroup DirectoryMode
  SocketMode Accept Writable FlushPending MaxConnections MaxConnectionsPerSource KeepAlive KeepAliveTimeSec
  KeepAliveIntervalSec KeepAliveProbes NoDelay Priority DeferAcceptSec ReceiveBuffer SendBuffer IPTOS IPTTL
  Mark ReusePort SmackLabel SmackLabelIPIn SmackLabelIPOut SELinuxContextFromNet PipeSize
  MessageQueueMaxMessages MessageQueueMessageSize FreeBind Transparent Broadcast PassCredentials
  PassSecurity PassPacketInfo Timestamping TCPCongestion Service RemoveOnStop Symlinks FileDescriptorName
  TriggerLimitIntervalSec TriggerLimitBurst PollLimitIntervalSec PollLimitBurst PassFileDescriptorsToExec
  DeferTrigger DeferTriggerMaxSec
  What Where Options SloppyOptions LazyUnmount ReadWriteOnly ForceUnmount ExtraOptions TimeoutIdleSec
  PathExists PathExistsGlob PathChanged PathModified DirectoryNotEmpty Unit MakeDirectory
  OnActiveSec OnBootSec OnStartupSec OnUnitActiveSec OnUnitInactiveSec OnCalendar AccuracySec
  RandomizedDelaySec RandomizedOffsetSec FixedRandomDelay OnClockChange OnTimezoneChange Persistent
  WakeSystem RemainAfterElapse DeferReactivation
  MACAddress PermanentMACAddress Path Driver Kind Property Name WLANInterfaceType SSID BSSID Host
  Virtualization KernelCommandLine KernelVersion Credential Architecture Firmware MACAddressPolicy NamePolicy
  AlternativeNamesPolicy AlternativeName MTUBytes BitsPerSecond Duplex AutoNegotiation WakeOnLan Port ARP
  Multicast AllMulticast Promiscuous Unmanaged RequiredForOnline RequiredFamilyForOnline ActivationPolicy
  DHCP DHCPServer LinkLocalAddressing IPv6LinkLocalAddressGenerationMode IPv4LLRoute DefaultRouteOnDevice
  LLMNR MulticastDNS DNSOverTLS DNSSEC DNSSECNegativeTrustAnchors LLDP EmitLLDP BindCarrier Address Gateway
  DNS Domains DNSDefaultRoute NTP IPForward IPv4Forwarding IPv6Forwarding IPMasquerade IPv6PrivacyExtensions
  IPv6AcceptRA IPv6DuplicateAddressDetection IPv6HopLimit IPv4ProxyARP IPv6ProxyNDP IPv6ProxyNDPAddress
  IPv6SendRA DHCPPrefixDelegation IPv6MTUBytes KeepMaster BatmanAdvanced Bond Bridge VRF IPVLAN IPVTAP L2TP
  MACsec MACVLAN MACVTAP Tunnel VLAN VXLAN Xfrm ActiveSlave PrimarySlave ConfigureWithoutCarrier
  IgnoreCarrierLoss KeepConfiguration Destination Source Metric Scope PreferredSource Table Protocol
  GatewayOnLink UseDNS UseNTP UseDomains UseRoutes UseHostname UseMTU UseGateway RouteMetric ClientIdentifier
  Anonymize SendHostname Hostname VendorClassIdentifier DUIDType DUIDRawData IAID RapidCommit Peer Label
  PreferredLifetime ValidLifetime HomeAddress DuplicateAddressDetection ManageTemporaryAddress
  AddPrefixRoute AutoJoin PoolOffset PoolSize EmitDNS EmitNTP EmitRouter DefaultLeaseTimeSec MaxLeaseTimeSec
  Id Mode PrivateKey PrivateKeyFile ListenPort FirewallMark RouteTable PublicKey PresharedKey
  PresharedKeyFile AllowedIPs Endpoint PersistentKeepalive Local Remote Independent
  Boot Ephemeral ProcessTwo Parameters PivotRoot Capability DropCapability MachineID NotifyReady ResolvConf
  Timezone LinkJournal ReadOnly Volatile Bind BindReadOnly BindUser Inaccessible Overlay OverlayReadOnly
  PrivateUsersOwnership Private VirtualEthernet VirtualEthernetExtra Interface Zone
  Image ContainerName Exec PublishPort Volume Network Label Annotation AutoUpdate Pod PodmanArgs Mount
  Secret HostName Entrypoint Notify Tmpfs AddCapability Device HealthCmd Ulimit WorkingDir UserNS Pull IP
  IP6 VolumeName PodName Yaml ImageTag File SetWorkingDirectory GlobalArgs ContainersConfModule`);

const SYSTEMD_BOOLEANS = words('yes no true false on off');

interface SystemdState {
  /** The previous line ended with "\", so this line continues a value. */
  continued: boolean;
  inValue: boolean;
  /** At the start of an Exec*= value, where prefixes like "-" or "@" may follow. */
  execPrefix: boolean;
}

export const systemd: StreamParser<SystemdState> = {
  name: 'systemd',
  startState: () => ({ continued: false, inValue: false, execPrefix: false }),
  token(stream, state) {
    if (stream.sol()) {
      const continued = state.continued;
      state.continued = false;
      state.inValue = continued;
      state.execPrefix = false;
      stream.eatSpace();
      if (stream.peek() === '#' || stream.peek() === ';') {
        // Comment lines inside a continued value are skipped by systemd.
        state.continued = continued;
        stream.skipToEnd();
        return 'comment';
      }
      if (!continued) {
        if (stream.match(/^\[[^\]]*\]/)) return 'heading';
        if (stream.match(/^[A-Za-z0-9_.\-]+(?=\s*=)/)) {
          const key = stream.current();
          state.execPrefix = /^Exec/.test(key);
          return SYSTEMD_KEYS.has(key) ? 'keyword' : 'propertyName';
        }
      }
    }
    if (stream.eatSpace()) return null;
    if (!state.inValue) {
      if (stream.eat('=')) {
        state.inValue = true;
        return 'operator';
      }
      stream.skipToEnd();
      return null;
    }
    if (stream.match(/^\\$/)) {
      state.continued = true;
      return 'operator';
    }
    if (state.execPrefix) {
      state.execPrefix = false;
      if (stream.match(/^[-@:+!|]+(?=\S)/)) return 'operator';
    }
    if (stream.match(/^%[a-zA-Z%]/)) return 'variableName.special';
    if (stream.match(/^\$\{\w+\}/) || stream.match(/^\$\w+/)) return 'variableName';
    if (stream.peek() === '"' || stream.peek() === "'") {
      const q = stream.next() as string;
      readQuoted(stream, q);
      return 'string';
    }
    if (stream.match(/^[^\s"'%$\\]+/)) {
      const w = stream.current();
      if (SYSTEMD_BOOLEANS.has(w.toLowerCase())) return 'atom';
      if (/^[+-]?\d+(\.\d+)?(%|[a-zA-Z]{1,5})?$/.test(w) || /^0x[0-9a-fA-F]+$/.test(w)) return 'number';
      if (/^[\w@:\-.\\]+\.(service|socket|device|mount|automount|swap|target|path|timer|slice|scope)$/.test(w)) {
        return 'typeName';
      }
      return null;
    }
    stream.next();
    return null;
  },
  languageData: { commentTokens: { line: '#' } },
};

// ---- Desktop entries (.desktop, .directory, mimeapps.list) ---------------------------------

const DESKTOP_KEYS = words(`Type Version Name GenericName NoDisplay Comment Icon Hidden OnlyShowIn NotShowIn
  DBusActivatable TryExec Exec Path Terminal Actions MimeType Categories Implements Keywords StartupNotify
  StartupWMClass URL PrefersNonDefaultGPU SingleMainWindow Encoding ServiceTypes`);

export const desktopEntry: StreamParser<{ inValue: boolean }> = {
  name: 'desktop',
  startState: () => ({ inValue: false }),
  token(stream, state) {
    if (stream.sol()) {
      state.inValue = false;
      stream.eatSpace();
      if (stream.peek() === '#') {
        stream.skipToEnd();
        return 'comment';
      }
      if (stream.match(/^\[[^\]]*\]/)) return 'heading';
      if (stream.match(/^[A-Za-z0-9-]+/)) return DESKTOP_KEYS.has(stream.current()) ? 'keyword' : 'propertyName';
    }
    if (!state.inValue) {
      if (stream.match(/^\[[^\]]*\]/)) return 'meta'; // locale, e.g. Name[de_DE]
      if (stream.eatSpace()) return null;
      if (stream.eat('=')) {
        state.inValue = true;
        return 'operator';
      }
      stream.skipToEnd();
      return null;
    }
    if (stream.eatSpace()) return null;
    if (stream.match(/^\\[sntr\;]/)) return 'escape';
    if (stream.match(/^%[fFuUdDnNickvm%]/)) return 'variableName.special';
    if (stream.eat(';')) return 'punctuation';
    if (stream.peek() === '"') {
      stream.next();
      readQuoted(stream, '"');
      return 'string';
    }
    if (stream.match(/^[^\s;\\%"]+/)) {
      const w = stream.current();
      if (w === 'true' || w === 'false') return 'atom';
      if (/^\d+(\.\d+)?$/.test(w)) return 'number';
      return null;
    }
    stream.next();
    return null;
  },
  languageData: { commentTokens: { line: '#' } },
};

// ---- KDL (v1 and v2) -------------------------------------------------------------------------

interface KdlState {
  /** Next identifier or string is a node name. */
  node: boolean;
  /** Depth of nested block comments. */
  comment: number;
  /** Closing delimiter of a string spanning lines ('' when none). */
  str: string;
  /** "/-" seen: the next node, argument, property or children block is commented out. */
  slashdash: boolean;
  /** Commented-out node or block in progress; brace depth inside it. */
  sdActive: boolean;
  sdDepth: number;
  /** The previous line ended with "\": no new node starts on this line. */
  continued: boolean;
}

const KDL_IDENT = /^[^\s\\/(){}[\]<>;=,"#][^\s\\/(){}[\]<>;=,"]*/;
const KDL_NUMBER = /^[+-]?(0x[0-9a-fA-F][0-9a-fA-F_]*|0o[0-7][0-7_]*|0b[01][01_]*|\d[\d_]*(\.\d[\d_]*)?([eE][+-]?\d[\d_]*)?)(?=[\s;{}()=\\/]|$)/;

/** Consumes string content up to and including `close`; returns false if the line ends first. */
function kdlStringBody(stream: StringStream, close: string): boolean {
  while (!stream.eol()) {
    if (close.startsWith('"') && close.length === 1 && stream.peek() === '\\') {
      stream.next();
      stream.next();
      continue;
    }
    if (stream.match(close)) return true;
    stream.next();
  }
  return false;
}

/** Consumes a (nestable) block comment up to its end or the end of the line. */
function kdlBlockComment(stream: StringStream, state: KdlState): string {
  while (!stream.eol()) {
    if (stream.match('/*')) state.comment++;
    else if (stream.match('*/')) {
      if (--state.comment === 0) break;
    } else stream.next();
  }
  return 'comment';
}

export const kdl: StreamParser<KdlState> = {
  name: 'kdl',
  startState: () => ({ node: true, comment: 0, str: '', slashdash: false, sdActive: false, sdDepth: 0, continued: false }),
  token(stream, state) {
    if (stream.sol()) {
      if (!state.continued && !state.str && !state.comment) {
        if (state.sdActive && state.sdDepth === 0) state.sdActive = false;
        if (!state.sdActive) state.node = true;
      }
      state.continued = false;
    }
    if (state.comment > 0) return kdlBlockComment(stream, state);
    if (state.str) {
      if (kdlStringBody(stream, state.str)) state.str = '';
      return state.sdActive ? 'comment' : 'string';
    }
    if (stream.eatSpace()) return null;
    if (stream.match('//')) {
      stream.skipToEnd();
      return 'comment';
    }
    if (stream.match('/*')) {
      state.comment = 1;
      return kdlBlockComment(stream, state);
    }
    if (stream.match('/-')) {
      state.slashdash = true;
      return 'comment';
    }
    if (stream.match(/^\\(?=\s*(\/\/.*)?$)/)) {
      state.continued = true;
      return 'operator';
    }

    // Commented-out node or children block: everything is a comment until it ends.
    if (state.sdActive) {
      const ch = stream.next() as string;
      if (ch === '{') state.sdDepth++;
      else if (ch === '}' && --state.sdDepth <= 0) {
        state.sdDepth = 0;
        state.sdActive = false;
        state.node = true;
      } else if (ch === ';' && state.sdDepth === 0) {
        state.sdActive = false;
        state.node = true;
      } else if (ch === '"') {
        if (!kdlStringBody(stream, '"')) state.str = '"';
      }
      return 'comment';
    }
    if (state.slashdash) {
      state.slashdash = false;
      if (state.node || stream.peek() === '{') {
        // A whole node (or a children block) is commented out.
        state.sdActive = true;
        state.sdDepth = 0;
        if (stream.eat('{')) state.sdDepth = 1;
        else state.node = false;
        stream.match(KDL_IDENT);
        return 'comment';
      }
      // A single argument or property.
      if (!stream.match(/^(\([^)]*\))?("(\\.|[^"\\])*"|[^\s;{}]+)/)) stream.next();
      return 'comment';
    }

    const ch = stream.peek();
    if (ch === ';') {
      stream.next();
      state.node = true;
      return 'punctuation';
    }
    if (ch === '{' || ch === '}') {
      stream.next();
      state.node = true;
      return 'brace';
    }
    if (stream.match(/^\([^)]*\)/)) return 'typeName';
    if (stream.eat('=')) return 'operator';

    const isNode = state.node;
    const nameStyle = () => {
      if (isNode) {
        state.node = false;
        return 'tagName';
      }
      return stream.match(/^\s*=/, false) ? 'propertyName' : null;
    };

    // Raw strings: r#"…"# (v1), #"…"# and #"""…"""# (v2).
    const raw = stream.match(/^r?(#+)("""|")/) as RegExpMatchArray | null;
    if (raw) {
      const close = raw[2] + raw[1];
      if (!kdlStringBody(stream, close)) state.str = close;
      return nameStyle() ?? 'string';
    }
    if (stream.match('"""')) {
      if (!kdlStringBody(stream, '"""')) state.str = '"""';
      return 'string';
    }
    if (stream.eat('"')) {
      if (!kdlStringBody(stream, '"')) state.str = '"';
      return nameStyle() ?? 'string';
    }
    if (stream.match(/^#(true|false|null|inf|-inf|nan)\b/)) return 'atom';
    if (!isNode && stream.match(KDL_NUMBER)) return 'number';
    if (stream.match(KDL_IDENT)) {
      const style = nameStyle();
      if (style) return style;
      return /^(true|false|null)$/.test(stream.current()) ? 'atom' : 'string';
    }
    stream.next();
    return null;
  },
  languageData: { commentTokens: { line: '//', block: { open: '/*', close: '*/' } } },
};
