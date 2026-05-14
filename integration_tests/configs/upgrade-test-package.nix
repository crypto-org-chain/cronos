let
  pkgs = import ../../nix { };
  # v1.0.15 pins nixpkgs release-22.11 whose default go is 1.19.8.
  # That binary is missing the LC_UUID load command required by macOS 15.
  # Inject go = pkgs.go into the attrs so selectGo short-circuits and
  # uses the current Go instead of scanning for a matching EOL version.
  buildGoApplicationWithGo =
    attrs:
    pkgs.buildGoApplication (
      attrs
      // {
        go = pkgs.go;
        modRoot = attrs.modRoot or ".";
      }
    );
  fetchFlake =
    repo: rev: sha256:
    (pkgs.flake-compat {
      src = {
        outPath = builtins.fetchTarball {
          url = "https://github.com/${repo}/archive/${rev}.tar.gz";
          inherit sha256;
        };
        inherit rev;
        shortRev = builtins.substring 0 7 rev;
      };
    }).defaultNix;
  # v1.0.15
  releasedGenesis =
    (fetchFlake "crypto-org-chain/cronos" "1f5e2618362303d91f621b47cbc1115cf4fa0195"
      "01pg64g89j2lphyxa2vdsjq2l40skprcaz9rq9sfiqksqzb50dzw"
    ).default.override
      {
        buildGoApplication = buildGoApplicationWithGo;
      };
  # release/v1.1.x
  released1_1 =
    (fetchFlake "crypto-org-chain/cronos" "69a80154b6b24fca15f3562e2c4b312ee1092220"
      "1996n5y6lla3w0hqcmcrn8wg0b6x43fh5raq0lm6k5jn0bcn37g2"
    ).default;
  # release/v1.2.x
  released1_2 =
    (fetchFlake "crypto-org-chain/cronos" "1aea999eef67a0a01b22422bad94b36e45b9759a"
      "0hgccz6i6c39bq9jwf57258dq9mqp86179g3im7i3jxsm302i97x"
    ).default;
  # release/v1.3.x
  released1_3 =
    (fetchFlake "crypto-org-chain/cronos" "dd3cea2df41732ef030a1f830244e340f3cf6bf0"
      "0n6g3ghb34ln29yb1av8gwif07dvjs6nrmq1a6hp0vjpn09090c3"
    ).default;
  # release/v1.4.8
  released1_4 =
    (fetchFlake "crypto-org-chain/cronos" "513fda768eb6d0602df1abe48abd4d2cda7a2a11"
      "1a7790r90f3zly82g9gj3kgscdghpv1sr29gs6nrb9qv5qcs2qrj"
    ).default;
  # release/v1.5.4
  released1_5 =
    (fetchFlake "crypto-org-chain/cronos" "5ccd423a14f100f4e485d0fb6aa8fa4b96d11b60"
      "0kd4i18v24ji64a06dc2ysjaiby5rbliymk7gngsfi1sn0qdqrn1"
    ).default;
  # release/v1.6.1
  released1_6 =
    (fetchFlake "crypto-org-chain/cronos" "05e102ef83b9ab0d5b55d46fb90f5fee53a295d2"
      "1cvyir2v00fv73yrzpdczlx2yl2lqy8j8qq9cblygnlghbxv1r9b"
    ).default;
  # release/v1.7.0
  released1_7 =
    (fetchFlake "crypto-org-chain/cronos" "40032610e530cc2c0c2fc83f104d6d19efa08ada"
      "0hqac845h690szany9rs8f4y9a12q95zlid82pcafd8a7776wakp"
    ).default;
  current = pkgs.callPackage ../../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  {
    name = "genesis";
    path = releasedGenesis;
  }
  {
    name = "v1.1.0";
    path = released1_1;
  }
  {
    name = "v1.2";
    path = released1_2;
  }
  {
    name = "v1.3";
    path = released1_3;
  }
  {
    name = "v1.4";
    path = released1_4;
  }
  {
    name = "v1.5";
    path = released1_5;
  }
  {
    name = "v1.6";
    path = released1_6;
  }
  {
    name = "v1.7";
    path = released1_7;
  }
  {
    name = "v1.8";
    path = current;
  }
]
