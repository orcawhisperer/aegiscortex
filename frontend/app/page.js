import Studio from "../components/Studio";
import { loadStudioBoot } from "../lib/backend";
import { parseStudioParams } from "../lib/permalink";

export const dynamic = "force-dynamic";

export default async function Page({ searchParams }) {
  const permalink = parseStudioParams(await searchParams);
  const boot = await loadStudioBoot(permalink.case);
  return <Studio boot={boot} permalink={permalink} />;
}
