import type { PropsWithChildren, ReactNode } from 'react';
import Card from '@mui/material/Card';
import CardContent from '@mui/material/CardContent';
import Divider from '@mui/material/Divider';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';

type PageCardProps = PropsWithChildren<{
  title: string;
  description?: string;
  action?: ReactNode;
}>

export default function PageCard({ title, description, action, children }: PageCardProps) {
  return (
    <Card elevation={0}>
      <CardContent sx={{ p: { xs: 2.5, md: 3 } }}>
        <Stack spacing={2.5}>
          <Stack
            direction={{ xs: 'column', sm: 'row' }}
            spacing={1.5}
            justifyContent="space-between"
            alignItems={{ xs: 'flex-start', sm: 'center' }}
          >
            <Stack spacing={0.5}>
              <Typography variant="h4">{title}</Typography>
              {description ? (
                <Typography variant="body2" color="text.secondary">
                  {description}
                </Typography>
              ) : null}
            </Stack>
            {action}
          </Stack>
          <Divider />
          <Stack spacing={2}>{children}</Stack>
        </Stack>
      </CardContent>
    </Card>
  );
}
