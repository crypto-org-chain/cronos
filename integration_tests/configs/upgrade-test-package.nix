let
  pkgs = import ../../nix { };
  # Map Nix system tuple → cronos release arch string.
  # Darwin x86_64 (Intel) is not supported — no release binaries since v1.7.0.
  arch =
    {
      "x86_64-linux" = "Linux_x86_64";
      "aarch64-linux" = "Linux_arm64";
      "aarch64-darwin" = "Darwin_arm64";
    }
    .${pkgs.stdenv.system} or (throw "unsupported system: ${pkgs.stdenv.system}");
  # Download a pre-built release binary instead of compiling from source.
  # pkgs.fetchzip strips the single top-level dir (bin/) so the result
  # contains just `cronosd` at its root.
  # On Linux the binary needs ELF repatching (autoPatchelfHook) to work outside
  # its original Nix closure; on Darwin the Mach-O binary runs as-is.
  fetchRelease =
    tag: version: sha256s:
    let
      src = pkgs.fetchzip {
        url = "https://github.com/crypto-org-chain/cronos/releases/download/${tag}/cronos_${version}_${arch}.tar.gz";
        sha256 = sha256s.${arch};
      };
    in
    pkgs.stdenv.mkDerivation {
      name = "cronos-release-${tag}";
      dontUnpack = true;
      nativeBuildInputs = pkgs.lib.optional pkgs.stdenv.isLinux pkgs.autoPatchelfHook;
      buildInputs = pkgs.lib.optionals pkgs.stdenv.isLinux (
        with pkgs;
        [
          stdenv.cc.cc.lib
          zlib
        ]
      );
      installPhase = ''
        mkdir -p $out/bin
        cp ${src}/cronosd $out/bin/cronosd
        chmod +x $out/bin/cronosd
      '';
    };
  current = pkgs.callPackage ../../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  {
    name = "genesis";
    path = fetchRelease "v1.0.15" "1.0.15" {
      Linux_x86_64 = "1ny07hv238la7l1nmbyxj0mi1xivdni5wipm8hp2hn203dyi1c23";
      Linux_arm64 = "0jvnlfdqxvlzqzcmfibkkxjy2a77qfkcg557y562rg8a380brgd2";
      Darwin_arm64 = "10w93r1b7d66yzgmwr3mj69j0n370xplwbh1zlds78ig41r5a4ir";
    };
  }
  {
    name = "v1.1.0";
    path = fetchRelease "v1.1.1" "1.1.1" {
      Linux_x86_64 = "1gq9wh911l51za3wx8xsph6nlmdn2b0arlyxhdk09w0ibb8llh6q";
      Linux_arm64 = "1mjnngssdbjdvbh6gbi1zgb775m0fnraryali9dr4mb385554n5g";
      Darwin_arm64 = "0y68gwbad7yij07frhgs1kmmgp0xkasfls5bvqq998hv1mqamhil";
    };
  }
  {
    name = "v1.2";
    path = fetchRelease "v1.2.0" "1.2.0" {
      Linux_x86_64 = "1gf0w22f21js48k8i9c2dyxzz5jd5zndklc060mymfrcicf286hl";
      Linux_arm64 = "13xcr1j5zz4p110mny0723dk5w03pkniysqck4hmnfmkq89c98f7";
      Darwin_arm64 = "1adyzvgwfbngfm4x4vik9imdjgrpgkxj4d40pmb45zkghj83gx8d";
    };
  }
  {
    name = "v1.3";
    path = fetchRelease "v1.3.4" "1.3.4" {
      Linux_x86_64 = "0hwdix4i5j4pd4373dmsfq5b7z6lwpz1c05yililiqfj292h54v8";
      Linux_arm64 = "0fb7svkpnd3rlyhhp5h3bdsdkhfr5w95fxw8ibm86b52pnpwg7a3";
      Darwin_arm64 = "0kjsmz7dwg18r11rxir2gygsn1xlh1sbnvfm30c6706hqj1a86g8";
    };
  }
  {
    name = "v1.4";
    path = fetchRelease "v1.4.8" "1.4.8" {
      Linux_x86_64 = "0g5kdpi2xbrwazrj4x123c44sz6sz1sk01h80mf802s7s709hard";
      Linux_arm64 = "1z9rh5j0jj6rqnwr1hwdvp7ci20jaaf7jqi01amxhds5g9in53sv";
      Darwin_arm64 = "14bmwpz4y63a2xqif3b6hpan5g9gj5pgqff5jzhd4p4cs2f2mk62";
    };
  }
  {
    name = "v1.5";
    path = fetchRelease "v1.5.4" "1.5.4" {
      Linux_x86_64 = "0zc5zpzkpf8gblrrcx34sr330kpk3p60bb4xi4qdpwz1jakj4md1";
      Linux_arm64 = "10lkyl1gm3g09zg1bmalr6q1ncy667rb571ai42nlj06p5kh2xsw";
      Darwin_arm64 = "1vspwlgva9pjk7pv74vgi1nr0bcgjwphwspdpr1mafqi6f94rrhn";
    };
  }
  {
    name = "v1.6";
    path = fetchRelease "v1.6.1" "1.6.1" {
      Linux_x86_64 = "1rypf0iwpib9pl7qghjv72sj5v7z5xj288jncg5qaply9v09fh57";
      Linux_arm64 = "0lg9lkj23ldpr15q4p3avicc4lkjvya07z033rsz5hn3rif62kg6";
      Darwin_arm64 = "15am423rq2lxydq7pprfj8bf9mx91cq22yn87aw9hd3lhx3g4zl9";
    };
  }
  {
    name = "v1.7";
    path = fetchRelease "v1.7.5" "1.7.5" {
      Linux_x86_64 = "0ay6xxlmjl6xl5bq4023v482s5999k9v0vg72hy8wm8hfmg97s5z";
      Linux_arm64 = "1hdh4f1x3zn0jcx4f76sd1k4v5ijijcphk4437lsl5mkpxzmyffk";
      Darwin_arm64 = "1hkwqsgkv1qqpcapqhg4vapwz3gxsv2j6sxqqaf3d9cpb7j418av";
    };
  }
  {
    name = "v1.8";
    path = current;
  }
]
