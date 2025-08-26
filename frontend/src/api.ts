export async function analyze(input: string) {
    const res = await fetch('/api/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ input }),
    });
    if (!res.ok) throw new Error('analyze failed');
    return res.json();
}


