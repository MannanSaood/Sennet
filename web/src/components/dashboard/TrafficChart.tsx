import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";

const data = [
    { time: '10:00', rx: 4000, tx: 2400 },
    { time: '10:05', rx: 3000, tx: 1398 },
    { time: '10:10', rx: 2000, tx: 9800 },
    { time: '10:15', rx: 2780, tx: 3908 },
    { time: '10:20', rx: 1890, tx: 4800 },
    { time: '10:25', rx: 2390, tx: 3800 },
    { time: '10:30', rx: 3490, tx: 4300 },
    { time: '10:35', rx: 4200, tx: 5100 },
    { time: '10:40', rx: 5100, tx: 6400 },
    { time: '10:45', rx: 5500, tx: 7100 },
    { time: '10:50', rx: 5900, tx: 6800 },
    { time: '10:55', rx: 5700, tx: 6200 },
];

export function TrafficChart() {
    return (
        <Card className="col-span-4 lg:col-span-3 border-dark-border bg-dark-surface/50">
            <CardHeader>
                <CardTitle>Network Traffic (Last Hour)</CardTitle>
            </CardHeader>
            <CardContent className="pl-2">
                <div className="h-[300px] w-full">
                    <ResponsiveContainer width="100%" height="100%">
                        <AreaChart data={data}>
                            <defs>
                                <linearGradient id="colorRx" x1="0" y1="0" x2="0" y2="1">
                                    <stop offset="5%" stopColor="#00D4FF" stopOpacity={0.3} />
                                    <stop offset="95%" stopColor="#00D4FF" stopOpacity={0} />
                                </linearGradient>
                                <linearGradient id="colorTx" x1="0" y1="0" x2="0" y2="1">
                                    <stop offset="5%" stopColor="#8B5CF6" stopOpacity={0.3} />
                                    <stop offset="95%" stopColor="#8B5CF6" stopOpacity={0} />
                                </linearGradient>
                            </defs>
                            <CartesianGrid strokeDasharray="3 3" stroke="#334155" vertical={false} />
                            <XAxis
                                dataKey="time"
                                stroke="#94a3b8"
                                fontSize={12}
                                tickLine={false}
                                axisLine={false}
                            />
                            <YAxis
                                stroke="#94a3b8"
                                fontSize={12}
                                tickLine={false}
                                axisLine={false}
                                tickFormatter={(value) => `${value / 1000}Gb`}
                            />
                            <Tooltip
                                contentStyle={{ backgroundColor: '#1e293b', borderColor: '#334155' }}
                                itemStyle={{ color: '#f8fafc' }}
                                labelStyle={{ color: '#94a3b8' }}
                            />
                            <Area
                                type="monotone"
                                dataKey="rx"
                                stroke="#00D4FF"
                                strokeWidth={2}
                                fillOpacity={1}
                                fill="url(#colorRx)"
                                name="Inbound (RX)"
                            />
                            <Area
                                type="monotone"
                                dataKey="tx"
                                stroke="#8B5CF6"
                                strokeWidth={2}
                                fillOpacity={1}
                                fill="url(#colorTx)"
                                name="Outbound (TX)"
                            />
                        </AreaChart>
                    </ResponsiveContainer>
                </div>
            </CardContent>
        </Card>
    );
}
