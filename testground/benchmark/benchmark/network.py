import ipaddress
from typing import List

import netifaces

from .params import RunParams


def get_data_ip(params: RunParams) -> ipaddress.IPv4Address:
    """
    Get the data network IP address
    """
    if not params.test_sidecar:
        return "127.0.0.1"

    for addr in ip4_addresses():
        if addr in params.test_subnet:
            return addr


def ip4_addresses() -> List[ipaddress.IPv4Address]:
    ip_list = []
    for interface in netifaces.interfaces():
        for link in netifaces.ifaddresses(interface).get(netifaces.AF_INET, []):
            ip_list.append(ipaddress.IPv4Address(link["addr"]))
    return ip_list
