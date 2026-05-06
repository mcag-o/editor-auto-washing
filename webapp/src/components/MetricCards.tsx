import type { ReactNode } from 'react';
import TrendingUpRoundedIcon from '@mui/icons-material/TrendingUpRounded';
import Box from '@mui/material/Box';
import Card from '@mui/material/Card';
import CardContent from '@mui/material/CardContent';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { alpha, useTheme } from '@mui/material/styles';

export type MetricCardItem = {
  key: string;
  label: string;
  value: string;
  hint?: string;
  icon?: ReactNode;
};

type MetricCardsProps = {
  items: MetricCardItem[];
};

export default function MetricCards({ items }: MetricCardsProps) {
  const theme = useTheme();

  return (
    <Box
      sx={{
        display: 'grid',
        gap: 2,
        gridTemplateColumns: {
          xs: '1fr',
          sm: 'repeat(2, minmax(0, 1fr))',
          xl: 'repeat(4, minmax(0, 1fr))',
        },
      }}
    >
      {items.map((item) => (
        <Card key={item.key} elevation={0}>
          <CardContent sx={{ p: 2.5 }}>
            <Stack spacing={1.5}>
              <Stack direction="row" justifyContent="space-between" alignItems="center">
                <Typography variant="body2" color="text.secondary">
                  {item.label}
                </Typography>
                <Box
                  sx={{
                    width: 36,
                    height: 36,
                    borderRadius: 2.5,
                    display: 'grid',
                    placeItems: 'center',
                    bgcolor: alpha(theme.palette.primary.main, 0.1),
                    color: 'primary.main',
                  }}
                >
                  {item.icon ?? <TrendingUpRoundedIcon fontSize="small" />}
                </Box>
              </Stack>
              <Typography variant="h3">{item.value}</Typography>
              {item.hint ? (
                <Typography variant="body2" color="text.secondary">
                  {item.hint}
                </Typography>
              ) : null}
            </Stack>
          </CardContent>
        </Card>
      ))}
    </Box>
  );
}
