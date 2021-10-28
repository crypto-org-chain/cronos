// THIS FILE IS GENERATED AUTOMATICALLY. DO NOT MODIFY.
import { SigningStargateClient } from "@cosmjs/stargate";
import { Registry } from "@cosmjs/proto-signing";
import { Api } from "./rest";
import { MsgConvertVouchers } from "./types/cronos/tx";
import { MsgTransferTokens } from "./types/cronos/tx";
import { MsgUpdateTokenMapping } from "./types/cronos/tx";
const types = [
    ["/cronos.MsgConvertVouchers", MsgConvertVouchers],
    ["/cronos.MsgTransferTokens", MsgTransferTokens],
    ["/cronos.MsgUpdateTokenMapping", MsgUpdateTokenMapping],
];
export const MissingWalletError = new Error("wallet is required");
const registry = new Registry(types);
const defaultFee = {
    amount: [],
    gas: "200000",
};
const txClient = async (wallet, { addr: addr } = { addr: "http://localhost:26657" }) => {
    if (!wallet)
        throw MissingWalletError;
    const client = await SigningStargateClient.connectWithSigner(addr, wallet, { registry });
    const { address } = (await wallet.getAccounts())[0];
    return {
        signAndBroadcast: (msgs, { fee, memo } = { fee: defaultFee, memo: "" }) => client.signAndBroadcast(address, msgs, fee, memo),
        msgConvertVouchers: (data) => ({ typeUrl: "/cronos.MsgConvertVouchers", value: data }),
        msgTransferTokens: (data) => ({ typeUrl: "/cronos.MsgTransferTokens", value: data }),
        msgUpdateTokenMapping: (data) => ({ typeUrl: "/cronos.MsgUpdateTokenMapping", value: data }),
    };
};
const queryClient = async ({ addr: addr } = { addr: "http://localhost:1317" }) => {
    return new Api({ baseUrl: addr });
};
export { txClient, queryClient, };
